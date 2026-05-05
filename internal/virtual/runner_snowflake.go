package virtual

import (
	"context"
	"fmt"
)

type SnowflakeRunner struct {
	dsn string
}

func NewSnowflakeRunner(dsn string) (*SnowflakeRunner, error) {
	return &SnowflakeRunner{dsn: dsn}, nil
}

func (r *SnowflakeRunner) Exec(ctx context.Context, query string) error {
	// fmt.Printf("[Snowflake Runner] Executing: %s\n", query)
	return nil
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
	return true, nil
}

func (r *SnowflakeRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	uniqueKey := config["unique_key"]
	if uniqueKey != "" {
		return fmt.Sprintf("MERGE INTO %s.%s t\nUSING (%s) s\nON t.%s = s.%s\nWHEN MATCHED THEN UPDATE SET *\nWHEN NOT MATCHED THEN INSERT *;", schema, table, selectSQL, uniqueKey, uniqueKey)
	}
	return fmt.Sprintf("INSERT INTO %s.%s\nSELECT * FROM (%s);", schema, table, selectSQL)
}

func (r *SnowflakeRunner) Name() string {
	return "snowflake"
}
