package parser

import (
	"testing"
)

// FuzzReplaceDependencies ensures the custom SQL tokenizer does not panic
// or hang when fed completely arbitrary, malformed, or malicious byte sequences.
// It acts as our property-based testing mechanism for the lexer.
func FuzzReplaceDependencies(f *testing.F) {
	// Provide a seed corpus covering the edge cases we care about
	f.Add("SELECT * FROM stg_users")
	f.Add("SELECT 'stg_users' FROM dual")
	f.Add("SELECT id AS stg_users FROM something_else")
	f.Add("WITH stg_users AS (SELECT 1) SELECT * FROM stg_users")
	f.Add("/* -- */ SELECT \x00\xff stg_users")
	f.Add("SELECT `stg_users` FROM \"stg_users\"")
	f.Add("")

	deps := map[string]string{
		"stg_users":  "sqlforge__fuzz.stg_users",
		"stg_orders": "sqlforge__fuzz.stg_orders",
	}

	f.Fuzz(func(t *testing.T, sql string) {
		// The only property we are asserting here is that the tokenizer
		// never panics or gets stuck in an infinite loop regardless of input.
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ReplaceDependencies panicked on input %q: %v", sql, r)
			}
		}()

		// Execute the tokenizer
		_ = ReplaceDependencies(sql, deps)
	})
}
