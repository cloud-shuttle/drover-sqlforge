package virtual

import (
	"context"
	"fmt"
)

type PostgresRunner struct {
	dsn string
}

func NewPostgresRunner(dsn string) (*PostgresRunner, error) {
	return &PostgresRunner{dsn: dsn}, nil
}

func (r *PostgresRunner) Exec(ctx context.Context, query string) error {
	// fmt.Printf("[Postgres Runner] Executing: %s\n", query)
	return nil
}

func (r *PostgresRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)
}

func (r *PostgresRunner) CreateTableDDL(schema, table, selectSQL string) string {
	// Postgres requires drop then create table as, or create table as.
	// For standard simplicity:
	return fmt.Sprintf("DROP TABLE IF EXISTS %s.%s CASCADE;\nCREATE TABLE %s.%s AS\n%s", schema, table, schema, table, selectSQL)
}

func (r *PostgresRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *PostgresRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("DROP MATERIALIZED VIEW IF EXISTS %s.%s CASCADE;\nCREATE MATERIALIZED VIEW %s.%s AS\n%s", schema, table, schema, table, selectSQL)
}

func (r *PostgresRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	// Postgres doesn't natively support Kafka engine tables
	return fmt.Sprintf("-- Postgres does not support native streaming engines for %s.%s", schema, table)
}

func (r *PostgresRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	return true, nil
}

func (r *PostgresRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	uniqueKey := config["unique_key"]
	if uniqueKey != "" {
		return fmt.Sprintf("INSERT INTO %s.%s\nSELECT * FROM (%s)\nON CONFLICT (%s) DO UPDATE SET *;", schema, table, selectSQL, uniqueKey)
	}
	return fmt.Sprintf("INSERT INTO %s.%s\nSELECT * FROM (%s);", schema, table, selectSQL)
}

func (r *PostgresRunner) Name() string {
	return "postgres"
}
