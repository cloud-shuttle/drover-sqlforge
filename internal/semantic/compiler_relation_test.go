package semantic

import (
	"strings"
	"testing"
)

func TestCompileUsesRelationWhenSet(t *testing.T) {
	m := &Metric{Name: "rpu", Expression: "SUM(revenue)", Model: "ab_test_results"}

	c := NewCompiler("sqlforge__prod")
	sql, err := c.Compile(m, nil)
	if err != nil || !strings.Contains(sql, "FROM sqlforge__prod.ab_test_results;") {
		t.Fatalf("default FROM: %v\n%s", err, sql)
	}

	c.Relation = "sqlforge__prod_marts.ab_test_results"
	sql, err = c.Compile(m, nil)
	if err != nil || !strings.Contains(sql, "FROM sqlforge__prod_marts.ab_test_results;") {
		t.Fatalf("relation FROM: %v\n%s", err, sql)
	}

	sql, _ = NewCompiler("").Compile(m, nil)
	if !strings.Contains(sql, "FROM ab_test_results;") {
		t.Fatalf("unqualified FROM:\n%s", sql)
	}
}
