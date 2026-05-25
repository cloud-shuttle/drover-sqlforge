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
	_ "github.com/go-sql-driver/mysql"
)

// VeloDBRunner implements virtual.Runner for VeloDB (SelectDB fork of Doris).
// VeloDB exposes the same MySQL-compatible wire protocol as Apache Doris.
type VeloDBRunner struct {
	dsn string
	db  *sql.DB
}

func (r *VeloDBRunner) Name() string { return "velodb" }

func (r *VeloDBRunner) Exec(ctx context.Context, query string) error {
	if r.db == nil {
		return fmt.Errorf("velodb connection not initialized (missing DSN)")
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// CreateSchemaDDL — VeloDB (like Doris) uses DATABASE semantics.
func (r *VeloDBRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", schema)
}

// CreateTableDDL — VeloDB CTAS. VeloDB inherits Doris's storage engine model
// (Duplicate/Aggregate/Unique Key). We default to a simple CTAS and let
// users add key model properties via post-apply DDL when needed.
func (r *VeloDBRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s\nPROPERTIES(\"replication_num\" = \"1\")\nAS\n%s", schema, table, selectSQL)
}

func (r *VeloDBRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

// CreateMaterializedViewDDL — VeloDB uses Doris-compatible MV syntax.
func (r *VeloDBRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE MATERIALIZED VIEW IF NOT EXISTS %s.%s AS\n%s", schema, table, selectSQL)
}

// CreateStreamingTableDDL — VeloDB supports Stream Load / Routine Load like Doris.
func (r *VeloDBRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf(
		"-- VeloDB streaming ingest requires Stream Load or Routine Load configuration for %s.%s.\n"+
			"-- Create the table first, then configure an ingestion job externally.",
		schema, table,
	)
}

func (r *VeloDBRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("velodb connection not initialized")
	}

	query := `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`

	// Strip catalog prefix (catalog.db → db).
	parts := strings.Split(schema, ".")
	searchSchema := parts[len(parts)-1]

	var count int
	err := r.db.QueryRowContext(ctx, query, searchSchema, table).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *VeloDBRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := virtual.BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *VeloDBRunner) QueryCount(ctx context.Context, query string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("velodb connection not initialized")
	}
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *VeloDBRunner) QueryData(ctx context.Context, query string) ([]map[string]interface{}, error) {
	if r.db == nil {
		return nil, fmt.Errorf("velodb connection not initialized")
	}
	rows, err := r.db.QueryContext(ctx, query)
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
	if dsn != "" && dsn != "stub" {
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("velodb plugin: failed to open connection: %v", err)
		}
		if err = db.Ping(); err != nil {
			log.Fatalf("velodb plugin: failed to ping: %v", err)
		}
		defer db.Close()
	}

	runner := &VeloDBRunner{dsn: dsn, db: db}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: virtual.Handshake,
		Plugins: map[string]plugin.Plugin{
			"runner": &virtual.RunnerGRPCPlugin{Impl: runner},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
