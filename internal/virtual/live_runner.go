package virtual

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type LiveRunner struct {
	db *sql.DB
}

func NewLiveRunner(dsn string) (*LiveRunner, error) {
	if dsn == "" {
		return nil, fmt.Errorf("connection string cannot be empty")
	}

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	return &LiveRunner{db: db}, nil
}

func (r *LiveRunner) Exec(ctx context.Context, query string) error {
	fmt.Printf("[ClickHouse Live] Executing: %s\n", query)
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *LiveRunner) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
