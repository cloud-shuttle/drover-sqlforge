package virtual

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresRunner struct {
	db   *sql.DB
	dsn  string
	stub bool
}

func NewPostgresRunner(dsn string) (*PostgresRunner, error) {
	if dsn == "" || dsn == "memory" || dsn == "stub" {
		return &PostgresRunner{stub: true}, nil
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &PostgresRunner{db: db, dsn: dsn, stub: false}, nil
}

func (r *PostgresRunner) Exec(ctx context.Context, query string) error {
	if r.stub {
		return nil
	}
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *PostgresRunner) CreateSchemaDDL(schema string) string {
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)
}

func (r *PostgresRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s.%s CASCADE;\nCREATE TABLE %s.%s AS\n%s", schema, table, schema, table, selectSQL)
}

func (r *PostgresRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s", schema, table, selectSQL)
}

func (r *PostgresRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return fmt.Sprintf("DROP MATERIALIZED VIEW IF EXISTS %s.%s CASCADE;\nCREATE MATERIALIZED VIEW %s.%s AS\n%s", schema, table, schema, table, selectSQL)
}

func (r *PostgresRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return fmt.Sprintf("-- Postgres does not support native streaming engines for %s.%s", schema, table)
}

func (r *PostgresRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	if r.stub {
		return true, nil
	}
	var exists int
	err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2", schema, table).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (r *PostgresRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	ddl, err := BuildIncrementalMergeDDL(r.Name(), schema, table, selectSQL, config)
	if err != nil {
		return fmt.Sprintf("-- error: %v", err)
	}
	return ddl
}

func (r *PostgresRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	if r.stub {
		return 0, nil
	}
	var count int
	err := r.db.QueryRowContext(ctx, sql).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *PostgresRunner) Name() string {
	return "postgres"
}

func (r *PostgresRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	if r.stub {
		return []map[string]interface{}{{"stub": "data"}}, nil
	}

	rows, err := r.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			if val != nil && *val != nil {
				m[colName] = *val
			} else {
				m[colName] = nil
			}
		}
		result = append(result, m)
	}

	return result, nil
}
