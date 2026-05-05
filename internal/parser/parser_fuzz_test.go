package parser

import (
	"context"
	"testing"
)

// FuzzParser tests the WASM parser boundary to ensure that malformed
// or enormous SQL queries do not crash the wazero runtime or Go process.
func FuzzParser(f *testing.F) {
	// Seed corpus with valid and invalid SQL
	f.Add("SELECT * FROM users")
	f.Add("SELECT * FROM")
	f.Add("DROP TABLE students; --")
	f.Add("SELECT \x00\x01\xff FROM nowhere")
	f.Add("")

	ctx := context.Background()
	p, err := NewParser(ctx)
	if err != nil {
		f.Fatalf("Failed to initialize parser: %v", err)
	}
	defer p.Close()

	f.Fuzz(func(t *testing.T, sql string) {
		// None of these should panic, even with garbage input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parser panicked on input %q: %v", sql, r)
			}
		}()

		// Call the mocked / WASM boundary methods
		_, _ = p.ParseToAST(sql)
		_, _ = p.Transpile(sql, "postgres", "clickhouse")
		_, _ = p.ExtractRefs(sql)
		_, _ = p.DetectTimePatterns(sql)
	})
}
