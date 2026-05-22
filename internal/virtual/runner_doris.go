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
	return fmt.Sprintf("-- Doris does not support native streaming engines for %s.%s", schema, table)
}

func (r *DorisRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	return true, nil
}

func (r *DorisRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *DorisRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	return 0, nil
}

func (r *DorisRunner) Name() string {
	return "doris"
}

func (r *DorisRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	return nil, nil
}
