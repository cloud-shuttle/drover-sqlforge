package parser

import (
	"context"
	"strings"
	"testing"
)

func TestTranspileWASM_Fallback(t *testing.T) {
	ctx := context.Background()
	p, err := NewParser(ctx)
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	defer p.Close()

	sql := "SELECT * FROM users"
	fromDialect := "snowflake"
	toDialect := "postgres"

	// Polyglot WASM doesn't export transpile_sql yet, so it should gracefully fall back
	// to the mocked Transpile function, which injects a comment.
	res, err := p.TranspileWASM(sql, fromDialect, toDialect)
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}

	if res == nil {
		t.Fatalf("expected non-nil result")
	}

	if !strings.Contains(res.SQL, "-- Transpiled from snowflake to postgres") {
		t.Errorf("expected fallback to inject transpile comment, got:\n%s", res.SQL)
	}
}
