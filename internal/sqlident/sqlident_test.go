package sqlident

import (
	"strings"
	"testing"
)

func TestValidateIdentifier(t *testing.T) {
	ok := []string{"orders", "_tmp", "sqlforge__peter_dev", "A1", strings.Repeat("a", 63)}
	bad := []string{"", "1abc", "a b", "a;b", "a.b", "a-b", "a'b", `a"b`, "a\nb", "a/*", "orders;DROP", strings.Repeat("a", 64), "é"}
	for _, s := range ok {
		if err := ValidateIdentifier("model", s); err != nil {
			t.Errorf("ValidateIdentifier(%q) = %v", s, err)
		}
	}
	for _, s := range bad {
		if err := ValidateIdentifier("model", s); err == nil {
			t.Errorf("ValidateIdentifier(%q) = nil, want error", s)
		}
	}
}

func TestValidateQualified(t *testing.T) {
	for _, s := range []string{"a", "a.b", "db.schema_x"} {
		if err := ValidateQualified("schema", s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"", "a.", ".a", "a..b", "a.b;c", "a. b"} {
		if err := ValidateQualified("schema", s); err == nil {
			t.Errorf("%q: want error", s)
		}
	}
}

func TestValidateEnvName(t *testing.T) {
	for _, s := range []string{"dev", "prod", "peter_dev", "pr_123", "Peter_Dev", strings.Repeat("a", 53)} {
		if err := ValidateEnvName(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"", "_dev", "1dev", "dev-1", "dev;DROP SCHEMA x", "dev x", strings.Repeat("a", 54)} {
		if err := ValidateEnvName(s); err == nil {
			t.Errorf("%q: want error", s)
		}
	}
}

func TestValidateColumnList(t *testing.T) {
	if err := ValidateColumnList("unique_key", "id, tenant_id"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"", " , ", "id; DROP TABLE x", "id, a b"} {
		if err := ValidateColumnList("unique_key", s); err == nil {
			t.Errorf("%q: want error", s)
		}
	}
}

func TestQuoteLiteral(t *testing.T) {
	cases := []struct{ dialect, in, want string }{
		{"postgres", "active", "'active'"},
		{"postgres", "it's", "'it''s'"},
		{"postgres", `a\`, `'a\'`},
		{"duckdb", `a\' OR 1=1 --`, `'a\'' OR 1=1 --'`},
		{"clickhouse", `a\' OR 1=1 --`, `'a\\'' OR 1=1 --'`},
		{"doris", `x\`, `'x\\'`},
		{"snowflake", "it's", "'it''s'"},
		{"postgres", "a\x00b", "'ab'"},
	}
	for _, c := range cases {
		if got := QuoteLiteral(c.dialect, c.in); got != c.want {
			t.Errorf("QuoteLiteral(%s, %q) = %s, want %s", c.dialect, c.in, got, c.want)
		}
	}
}
