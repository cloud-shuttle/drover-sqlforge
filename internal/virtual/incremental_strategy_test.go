package virtual

import (
	"strings"
	"testing"
)

func TestResolveIncrementalStrategy(t *testing.T) {
	tests := []struct {
		config map[string]string
		want   string
	}{
		{map[string]string{}, StrategyAppend},
		{map[string]string{"incremental_strategy": "auto"}, StrategyAppend},
		{map[string]string{"incremental_strategy": "auto", "unique_key": "id"}, StrategyMerge},
		{map[string]string{"incremental_strategy": "append"}, StrategyAppend},
		{map[string]string{"incremental_strategy": "upsert", "unique_key": "id"}, StrategyMerge},
		{map[string]string{"incremental_strategy": "delete+insert", "unique_key": "id"}, StrategyDeleteInsert},
	}
	for _, tt := range tests {
		if got := ResolveIncrementalStrategy(tt.config); got != tt.want {
			t.Errorf("config %v: got %q want %q", tt.config, got, tt.want)
		}
	}
}

func TestBuildIncrementalMergeDeleteInsert(t *testing.T) {
	ddl, err := BuildIncrementalMergeDDL("postgres", "db", "t", "SELECT 1 AS id", map[string]string{
		"incremental_strategy": "delete+insert",
		"unique_key":           "id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "DELETE FROM") || !strings.Contains(ddl, "INSERT INTO") {
		t.Fatalf("unexpected ddl: %s", ddl)
	}
}

func TestBuildIncrementalMergeDDL_TrimsTerminator(t *testing.T) {
	ddl, err := BuildIncrementalMergeDDL("postgres", "s", "t", "SELECT 'a;b' AS name;", map[string]string{
		"incremental_strategy": "append",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ddl, "name;") {
		t.Fatalf("terminator left inside subquery: %s", ddl)
	}
	if !strings.Contains(ddl, "SELECT 'a;b' AS name)") {
		t.Fatalf("quoted semicolon lost: %s", ddl)
	}

	ddl, err = BuildIncrementalMergeDDL("duckdb", "s", "t", "SELECT 'a;b' AS name;", map[string]string{
		"incremental_strategy": "merge",
		"unique_key":           "id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ddl, "name;") || !strings.Contains(ddl, "DELETE FROM") {
		t.Fatalf("merge ddl = %s", ddl)
	}
}

func TestBuildIncrementalInitialReplacingMergeTree(t *testing.T) {
	ddl, err := BuildIncrementalInitialDDL("clickhouse", "db", "t", "SELECT 1 AS id, now() AS updated_at", map[string]string{
		"incremental_strategy": "replacing_merge_tree",
		"unique_key":           "id",
		"updated_at":           "updated_at",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "ReplacingMergeTree") {
		t.Fatalf("expected ReplacingMergeTree, got: %s", ddl)
	}
}
