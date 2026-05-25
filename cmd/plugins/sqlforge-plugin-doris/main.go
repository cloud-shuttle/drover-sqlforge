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

// DorisRunner implements virtual.Runner for Apache Doris.
// Doris exposes a MySQL-compatible wire protocol, so we connect via
// go-sql-driver/mysql and generate Doris-dialect DDL.
type DorisRunner struct {
	dsn string
	db  *sql.DB
}

func (r *DorisRunner) Name() string { return "doris" }

func (r *DorisRunner) Exec(ctx context.Context, query string) error {
	if r.db == nil {
		return fmt.Errorf("doris connection not initialized (missing DSN)")
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// CreateSchemaDDL — Doris uses DATABASE instead of SCHEMA.
func (r *DorisRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", schema)
}

// CreateTableDDL — Doris CTAS with Duplicate Key as the default engine.
// When the selectSQL result is known at compile time we emit standard CTAS;
// Doris will infer the schema. Users needing a specific key model should set
// it in model config post-apply.
func (r *DorisRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s\nPROPERTIES(\"replication_num\" = \"1\")\nAS\n%s", schema, table, selectSQL)
}

func (r *DorisRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE VIEW IF NOT EXISTS %s.%s AS\n%s", schema, table, selectSQL)
}

// CreateMaterializedViewDDL — Doris materialized views are tied to a base table;
// here we generate a synchronous materialized view refresh from selectSQL.
func (r *DorisRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE MATERIALIZED VIEW %s\nAS\n%s", table, selectSQL)
}

// CreateStreamingTableDDL — Doris supports Routine Load / Stream Load but not
// a single DDL statement for streaming ingest. Emit a comment so plan output
// is transparent.
func (r *DorisRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf(
		"-- Doris streaming ingest requires Routine Load or Stream Load configuration for %s.%s.\n"+
			"-- Create the table first, then configure an ingestion job externally.",
		schema, table,
	)
}

func (r *DorisRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("doris connection not initialized")
	}

	// Doris surfaces information_schema like MySQL.
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

func (r *DorisRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := virtual.BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *DorisRunner) QueryCount(ctx context.Context, query string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("doris connection not initialized")
	}
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *DorisRunner) QueryData(ctx context.Context, query string) ([]map[string]interface{}, error) {
	if r.db == nil {
		return nil, fmt.Errorf("doris connection not initialized")
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
			log.Fatalf("doris plugin: failed to open connection: %v", err)
		}
		if err = db.Ping(); err != nil {
			log.Fatalf("doris plugin: failed to ping: %v", err)
		}
		defer db.Close()
	}

	runner := &DorisRunner{dsn: dsn, db: db}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: virtual.Handshake,
		Plugins: map[string]plugin.Plugin{
			"runner": &virtual.RunnerGRPCPlugin{Impl: runner},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
