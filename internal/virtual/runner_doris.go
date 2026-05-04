package virtual

import (
	"context"
	"fmt"
)

type DorisRunner struct {
	dsn string
}

func NewDorisRunner(dsn string) (*DorisRunner, error) {
	return &DorisRunner{dsn: dsn}, nil
}

func (r *DorisRunner) Exec(ctx context.Context, query string) error {
	// fmt.Printf("[Doris Runner] Executing: %s\n", query)
	return nil
}

func (r *DorisRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", schema)
}

func (r *DorisRunner) CreateTableDDL(schema, table, selectSQL string) string {
	// Doris uses CTAS with engine specified if needed, but standard CTAS works
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DorisRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE VIEW IF NOT EXISTS %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *DorisRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE MATERIALIZED VIEW %s AS\n%s", table, selectSQL)
}

func (r *DorisRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf("-- Doris requires Routine Load for Kafka streaming for %s.%s", schema, table)
}

func (r *DorisRunner) Name() string {
	return "doris"
}
