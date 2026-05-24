package snapshot

import (
	"strings"
	"testing"
)

func TestBuildRunInitial(t *testing.T) {
	stmts, err := BuildRun("duckdb", "sqlforge__prod", "users_snapshot", false,
		"SELECT 1 AS id", ResolvedConfig{Strategy: "timestamp", UniqueKey: "id", UpdatedAt: "updated_at"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], ValidFrom) || !strings.Contains(stmts[0], "CREATE TABLE") {
		t.Errorf("unexpected initial DDL: %s", stmts[0])
	}
}

func TestBuildRunTimestampIncremental(t *testing.T) {
	stmts, err := BuildRun("postgres", "sqlforge__prod", "users_snapshot", true,
		"SELECT 1 AS id", ResolvedConfig{Strategy: "timestamp", UniqueKey: "id", UpdatedAt: "updated_at"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 4 {
		t.Fatalf("expected 4 statements, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[1], "UPDATE") || !strings.Contains(stmts[1], ValidTo) {
		t.Errorf("expected close-current UPDATE, got: %s", stmts[1])
	}
	if !strings.Contains(stmts[2], "INSERT INTO") {
		t.Errorf("expected INSERT, got: %s", stmts[2])
	}
}

func TestBuildRunClickHouseAppend(t *testing.T) {
	stmts, err := BuildRun("clickhouse", "sqlforge__prod", "users_snapshot", true,
		"SELECT 1 AS id", ResolvedConfig{Strategy: "timestamp", UniqueKey: "id", UpdatedAt: "updated_at"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 || !strings.Contains(stmts[0], "INSERT INTO") {
		t.Fatalf("expected single INSERT for clickhouse, got: %v", stmts)
	}
}

func TestResolveConfigRequiresUniqueKey(t *testing.T) {
	_, err := ResolveConfig(&Definition{Name: "x", Config: map[string]string{"updated_at": "t"}})
	if err == nil {
		t.Fatal("expected error without unique_key")
	}
}

func TestResolveConfigCheck(t *testing.T) {
	cfg, err := ResolveConfig(&Definition{
		Name:   "x",
		Config: map[string]string{"strategy": "check", "unique_key": "id", "check_cols": "status, amount"},
	})
	if err != nil {
		t.Fatalf("unexpected error for check strategy: %v", err)
	}
	if len(cfg.CheckCols) != 2 || cfg.CheckCols[0] != "status" || cfg.CheckCols[1] != "amount" {
		t.Errorf("failed to parse check_cols: %v", cfg.CheckCols)
	}
}

func TestResolveConfigCheckRequiresCols(t *testing.T) {
	_, err := ResolveConfig(&Definition{
		Name:   "x",
		Config: map[string]string{"strategy": "check", "unique_key": "id"},
	})
	if err == nil {
		t.Fatal("expected error when check_cols is missing")
	}
}

func TestBuildRunCheckIncremental(t *testing.T) {
	stmts, err := BuildRun("postgres", "sqlforge__prod", "users_snapshot", true,
		"SELECT 1 AS id, 'active' AS status", ResolvedConfig{Strategy: "check", UniqueKey: "id", CheckCols: []string{"status"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[1], "UPDATE") || !strings.Contains(stmts[1], "IS DISTINCT FROM") {
		t.Errorf("expected UPDATE with IS DISTINCT FROM, got: %s", stmts[1])
	}
	if !strings.Contains(stmts[2], "INSERT INTO") || !strings.Contains(stmts[2], "IS DISTINCT FROM") {
		t.Errorf("expected INSERT with IS DISTINCT FROM, got: %s", stmts[2])
	}
}
