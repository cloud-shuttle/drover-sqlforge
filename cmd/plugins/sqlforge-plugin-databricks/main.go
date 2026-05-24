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
	_ "github.com/databricks/databricks-sql-go"
)

type DatabricksRunner struct {
	dsn string
	db  *sql.DB
}

func (r *DatabricksRunner) Exec(ctx context.Context, query string) error {
	if r.db == nil {
		return fmt.Errorf("databricks connection not initialized (missing DSN)")
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *DatabricksRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)
}

func (r *DatabricksRunner) CreateTableDDL(schema, table, selectSQL string) string {
	// Databricks supports CREATE OR REPLACE TABLE via Delta
	return fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s USING DELTA AS\n%s", schema, table, selectSQL)
}

func (r *DatabricksRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DatabricksRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE MATERIALIZED VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DatabricksRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf("-- Databricks requires Delta Live Tables for native streaming %s.%s", schema, table)
}

func (r *DatabricksRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("databricks connection not initialized")
	}

	query := `SELECT COUNT(*) FROM system.information_schema.tables WHERE table_schema = ? AND table_name = ?`
	
	// Handle db.schema format
	parts := strings.Split(schema, ".")
	searchSchema := schema
	if len(parts) > 1 {
		searchSchema = parts[1]
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, searchSchema, table).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *DatabricksRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := virtual.BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *DatabricksRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("databricks connection not initialized")
	}
	var count int
	err := r.db.QueryRowContext(ctx, sql).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *DatabricksRunner) Name() string {
	return "databricks"
}

func (r *DatabricksRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	if r.db == nil {
		return nil, fmt.Errorf("databricks connection not initialized")
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
		db, err = sql.Open("databricks", dsn)
		if err != nil {
			log.Fatalf("failed to connect to databricks: %v", err)
		}
		defer db.Close()
	}

	runner := &DatabricksRunner{dsn: dsn, db: db}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: virtual.Handshake,
		Plugins: map[string]plugin.Plugin{
			"runner": &virtual.RunnerGRPCPlugin{Impl: runner},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
