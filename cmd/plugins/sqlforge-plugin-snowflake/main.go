package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/virtual"
	"github.com/hashicorp/go-plugin"
	_ "github.com/snowflakedb/gosnowflake"
)

type SnowflakeRunner struct {
	dsn string
	db  *sql.DB
}

func (r *SnowflakeRunner) Exec(ctx context.Context, query string) error {
	if r.db == nil {
		return fmt.Errorf("snowflake connection not initialized (missing DSN)")
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *SnowflakeRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)
}

func (r *SnowflakeRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *SnowflakeRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *SnowflakeRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE MATERIALIZED VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *SnowflakeRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf("-- Snowflake does not support native streaming engines for %s.%s", schema, table)
}

func (r *SnowflakeRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	// For Snowflake, schema and table names are generally uppercase unless quoted.
	// But INFORMATION_SCHEMA queries can handle strings.
	query := `SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = CURRENT_SCHEMA() AND TABLE_NAME = ?`
	// Assuming `schema` might contain db.schema, we simplify to checking within current context for now,
	// or querying with ILIKE. Let's use ILIKE for flexibility.
	query = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA ILIKE ? AND TABLE_NAME ILIKE ?`
	
	// Handle db.schema format
	parts := strings.Split(schema, ".")
	searchSchema := schema
	if len(parts) > 1 {
		searchSchema = parts[1]
	}

	if r.db == nil {
		return false, fmt.Errorf("snowflake connection not initialized")
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, searchSchema, table).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SnowflakeRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := virtual.BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *SnowflakeRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("snowflake connection not initialized")
	}
	var count int
	err := r.db.QueryRowContext(ctx, sql).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SnowflakeRunner) Name() string {
	return "snowflake"
}

func (r *SnowflakeRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	if r.db == nil {
		return nil, fmt.Errorf("snowflake connection not initialized")
	}
	rows, err := r.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			rowMap[colName] = *val
		}
		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func main() {
	dsn := os.Getenv("SQLFORGE_PLUGIN_DSN")
	
	var db *sql.DB
	var err error
	if dsn != "" {
		db, err = sql.Open("snowflake", dsn)
		if err != nil {
			log.Fatalf("failed to connect to snowflake: %v", err)
		}
		defer db.Close()
	}

	runner := &SnowflakeRunner{dsn: dsn, db: db}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: virtual.Handshake,
		Plugins: map[string]plugin.Plugin{
			"runner": &virtual.RunnerGRPCPlugin{Impl: runner},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
