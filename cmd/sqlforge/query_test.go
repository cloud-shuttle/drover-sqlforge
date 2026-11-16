package main

import (
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
)

func TestMetricRelation(t *testing.T) {
	assets := []*model.Asset{
		{Name: "ab_test_results", Config: map[string]string{"schema": "marts"}},
		{Name: "orders", Config: map[string]string{"alias": "fct_orders", "database": "analytics"}},
		{Name: "users", Config: map[string]string{}},
	}
	cases := map[string]string{
		"ab_test_results": "sqlforge__prod_marts.ab_test_results",
		"orders":          "analytics.sqlforge__prod.fct_orders",
		"users":           "sqlforge__prod.users",
		"missing":         "",
	}
	for name, want := range cases {
		if got := metricRelation("sqlforge__prod", assets, name); got != want {
			t.Errorf("metricRelation(%q) = %q, want %q", name, got, want)
		}
	}
}
