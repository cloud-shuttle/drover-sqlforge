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
