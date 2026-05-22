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
	// fmt.Printf("[Databricks Runner] Executing: %s\n", query)
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
	return fmt.Sprintf("-- Databricks requires Delta Live Tables for native streaming %s.%s", schema, table)
}

func (r *DatabricksRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	return true, nil
}

func (r *DatabricksRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *DatabricksRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	return 0, nil
}

func (r *DatabricksRunner) Name() string {
	return "databricks"
}

func (r *DatabricksRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	return nil, nil
}
