package virtual

import (
	"context"
	"fmt"
)

type VeloDBRunner struct {
	dsn string
}

func NewVeloDBRunner(dsn string) (*VeloDBRunner, error) {
	return &VeloDBRunner{dsn: dsn}, nil
}

func (r *VeloDBRunner) Exec(ctx context.Context, query string) error {
	fmt.Printf("[VeloDB Runner] Executing: %s\n", query)
	return nil
}

func (r *VeloDBRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", schema)
}

func (r *VeloDBRunner) CreateTableDDL(schema, table, selectSQL string) string {
	// Similar to ClickHouse / Doris
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *VeloDBRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *VeloDBRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *VeloDBRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf("-- VeloDB does not support native streaming engines for %s.%s", schema, table)
}

func (r *VeloDBRunner) Name() string {
	return "velodb"
}
