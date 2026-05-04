package virtual

import (
	"context"
	"fmt"
)

type DuckDBRunner struct {
	dsn string
}

func NewDuckDBRunner(dsn string) (*DuckDBRunner, error) {
	// For MVP, DuckDB is implemented as a stub that prints DDL
	return &DuckDBRunner{dsn: dsn}, nil
}

func (r *DuckDBRunner) Exec(ctx context.Context, query string) error {
	fmt.Printf("[DuckDB Runner] Executing: %s\n", query)
	return nil
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
	// DuckDB doesn't have native materialized views, so we fall back to CREATE TABLE AS
	return fmt.Sprintf("CREATE OR REPLACE TABLE %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DuckDBRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	// DuckDB doesn't support Kafka streaming engines natively
	return fmt.Sprintf("-- DuckDB does not support native streaming engines for %s.%s", schema, table)
}

func (r *DuckDBRunner) Name() string {
	return "duckdb"
}
