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
	_ "github.com/marcboeker/go-duckdb"
)

type DuckDBRunner struct {
	dsn string
	db  *sql.DB
}

func (r *DuckDBRunner) Exec(ctx context.Context, query string) error {
	if r.db == nil {
		return fmt.Errorf("duckdb connection not initialized")
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *DuckDBRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)
}

func (r *DuckDBRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DuckDBRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DuckDBRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DuckDBRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf("-- DuckDB does not support native streaming engines for %s.%s", schema, table)
}

func (r *DuckDBRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("duckdb connection not initialized")
	}

	query := `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`
	
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

func (r *DuckDBRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := virtual.BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *DuckDBRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("duckdb connection not initialized")
	}
	var count int
	err := r.db.QueryRowContext(ctx, sql).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *DuckDBRunner) Name() string {
	return "duckdb"
}

func (r *DuckDBRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	if r.db == nil {
		return nil, fmt.Errorf("duckdb connection not initialized")
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

func registerClickHouseCompatibility(db *sql.DB) error {
	_, _ = db.Exec("CREATE SCHEMA IF NOT EXISTS system;")
	_, _ = db.Exec("CREATE OR REPLACE VIEW system.numbers AS SELECT range AS number FROM range(10000);")
	_, _ = db.Exec("CREATE OR REPLACE FUNCTION toString(x) AS CAST(x AS VARCHAR);")
	_, _ = db.Exec("CREATE OR REPLACE FUNCTION now() AS CAST(current_timestamp AS TIMESTAMP);")
	_, _ = db.Exec("CREATE OR REPLACE FUNCTION toIntervalMinute(x) AS INTERVAL (x) MINUTE;")
	_, _ = db.Exec("CREATE OR REPLACE FUNCTION toIntervalDay(x) AS INTERVAL (x) DAY;")
	_, _ = db.Exec("CREATE OR REPLACE FUNCTION multiIf(cond, trueVal, falseVal) AS CASE WHEN cond THEN trueVal ELSE falseVal END;")
	_, _ = db.Exec("CREATE OR REPLACE FUNCTION \"if\"(cond, trueVal, falseVal) AS CASE WHEN cond THEN trueVal ELSE falseVal END;")
	return nil
}

func main() {
	dsn := os.Getenv("SQLFORGE_PLUGIN_DSN")

	var db *sql.DB
	var err error
	if dsn != "" {
		dbPath := dsn
		if dsn == "memory" || dsn == "stub" || dsn == ":memory:" {
			dbPath = ""
		}
		db, err = sql.Open("duckdb", dbPath)
		if err != nil {
			log.Fatalf("failed to connect to duckdb: %v", err)
		}
		defer db.Close()

		if err := registerClickHouseCompatibility(db); err != nil {
			log.Fatalf("failed to register clickhouse compatibility for duckdb: %v", err)
		}
	}

	runner := &DuckDBRunner{dsn: dsn, db: db}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: virtual.Handshake,
		Plugins: map[string]plugin.Plugin{
			"runner": &virtual.RunnerGRPCPlugin{Impl: runner},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
