package snapshot

import "testing"

func TestBuildRun_RejectsUnsafeIdentifiers(t *testing.T) {
	ok := ResolvedConfig{Strategy: "check", UniqueKey: "id", UpdatedAt: "updated_at", CheckCols: []string{"email"}}
	if _, err := BuildRun("postgres", "sqlforge__prod", "users_snapshot", true, "SELECT 1", ok); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := []struct {
		name, schema, table string
		cfg                 ResolvedConfig
	}{
		{"schema", "sqlforge__prod; DROP SCHEMA x", "s", ok},
		{"table", "sqlforge__prod", "s; DROP TABLE x", ok},
		{"unique_key", "sqlforge__prod", "s", ResolvedConfig{Strategy: "timestamp", UniqueKey: "id) OR (1=1", UpdatedAt: "u"}},
		{"updated_at", "sqlforge__prod", "s", ResolvedConfig{Strategy: "timestamp", UniqueKey: "id", UpdatedAt: "u--"}},
		{"check_cols", "sqlforge__prod", "s", ResolvedConfig{Strategy: "check", UniqueKey: "id", CheckCols: []string{"a", "b c"}}},
	}
	for _, c := range cases {
		for _, exists := range []bool{false, true} {
			if _, err := BuildRun("clickhouse", c.schema, c.table, exists, "SELECT 1", c.cfg); err == nil {
				t.Errorf("%s (exists=%v): want error", c.name, exists)
			}
		}
	}
}
