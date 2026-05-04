package virtual

import (
	"context"
	"fmt"
)

type DatabricksRunner struct {
	dsn string
}

func NewDatabricksRunner(dsn string) (*DatabricksRunner, error) {
	return &DatabricksRunner{dsn: dsn}, nil
}

func (r *DatabricksRunner) Exec(ctx context.Context, query string) error {
	fmt.Printf("[Databricks Runner] Executing: %s\n", query)
	return nil
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
	return fmt.Sprintf("-- Databricks streaming tables handled via DLT for %s.%s", schema, table)
}

func (r *DatabricksRunner) Name() string {
	return "databricks"
}
