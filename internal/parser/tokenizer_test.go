package parser

import (
	"testing"
)

func TestReplaceDependencies(t *testing.T) {
	deps := map[string]string{
		"stg_users": "sqlforge__dev.stg_users",
		"stg_orders": "sqlforge__dev.stg_orders",
	}

	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "Basic FROM",
			sql:      "SELECT * FROM stg_users",
			expected: "SELECT * FROM sqlforge__dev.stg_users",
		},
		{
			name:     "Basic JOIN",
			sql:      "SELECT * FROM stg_users JOIN stg_orders ON 1=1",
			expected: "SELECT * FROM sqlforge__dev.stg_users JOIN sqlforge__dev.stg_orders ON 1=1",
		},
		{
			name:     "Ignore String Literals",
			sql:      "SELECT 'stg_users' AS name FROM stg_users",
			expected: "SELECT 'stg_users' AS name FROM sqlforge__dev.stg_users",
		},
		{
			name:     "Ignore Line Comments",
			sql:      "SELECT * FROM stg_users -- don't replace stg_orders",
			expected: "SELECT * FROM sqlforge__dev.stg_users -- don't replace stg_orders",
		},
		{
			name:     "Ignore Block Comments",
			sql:      "SELECT * FROM stg_users /* stg_orders */",
			expected: "SELECT * FROM sqlforge__dev.stg_users /* stg_orders */",
		},
		{
			name:     "Handle Column Identifiers",
			sql:      "SELECT stg_users.id FROM stg_users",
			expected: "SELECT sqlforge__dev.stg_users.id FROM sqlforge__dev.stg_users",
		},
		{
			name:     "Handle Comma Separated Tables",
			sql:      "SELECT * FROM stg_users, stg_orders",
			expected: "SELECT * FROM sqlforge__dev.stg_users, sqlforge__dev.stg_orders",
		},
		{
			name:     "Ignore Alias",
			sql:      "SELECT id AS stg_users FROM stg_orders",
			expected: "SELECT id AS stg_users FROM sqlforge__dev.stg_orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceDependencies(tt.sql, deps)
			if got != tt.expected {
				t.Errorf("ReplaceDependencies() = %v, want %v", got, tt.expected)
			}
		})
	}
}
