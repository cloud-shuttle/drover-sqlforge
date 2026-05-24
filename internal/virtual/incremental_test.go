package virtual

import (
	"strings"
	"testing"
)

func TestCreateIncrementalMergeDDL(t *testing.T) {
	schema := "test_db"
	table := "test_table"
	selectSQL := "SELECT 1 AS id, 'a' AS val"

	tests := []struct {
		name     string
		dialect  string
		config   map[string]string
		expected string
	}{
		{
			name:     "DuckDB Append",
			dialect:  "duckdb",
			config:   map[string]string{},
			expected: "INSERT INTO test_db.test_table\nSELECT * FROM (SELECT 1 AS id, 'a' AS val);",
		},
		{
			name:     "DuckDB Upsert",
			dialect:  "duckdb",
			config:   map[string]string{"unique_key": "id"},
			expected: "INSERT INTO test_db.test_table\nSELECT * FROM (SELECT 1 AS id, 'a' AS val)\nON CONFLICT (id) DO UPDATE SET *;",
		},
		{
			name:     "Snowflake Append",
			dialect:  "snowflake",
			config:   map[string]string{},
			expected: "INSERT INTO test_db.test_table\nSELECT * FROM (SELECT 1 AS id, 'a' AS val);",
		},
		{
			name:     "Snowflake Upsert",
			dialect:  "snowflake",
			config:   map[string]string{"unique_key": "id"},
			expected: "MERGE INTO test_db.test_table t\nUSING (SELECT 1 AS id, 'a' AS val) s\nON t.id = s.id\nWHEN MATCHED THEN UPDATE SET *\nWHEN NOT MATCHED THEN INSERT *;",
		},
		{
			name:     "ClickHouse Append",
			dialect:  "clickhouse",
			config:   map[string]string{"unique_key": "id"}, // ClickHouse ignores unique_key for the merge DDL
			expected: "INSERT INTO test_db.test_table\nSELECT * FROM (SELECT 1 AS id, 'a' AS val);",
		},
		{
			name:     "Postgres Upsert",
			dialect:  "postgres",
			config:   map[string]string{"unique_key": "id"},
			expected: "INSERT INTO test_db.test_table\nSELECT * FROM (SELECT 1 AS id, 'a' AS val)\nON CONFLICT (id) DO UPDATE SET *;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildIncrementalMergeDDL(tt.dialect, schema, table, selectSQL, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected DDL to contain:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}
