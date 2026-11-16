package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/state"
)

// clickhouseCountRunner reports a ClickHouse dialect so literal escaping
// doubles backslashes, and returns a per-call count.
type clickhouseCountRunner struct {
	MockRunner
	counts []int
}

func (r *clickhouseCountRunner) Name() string { return "clickhouse" }
func (r *clickhouseCountRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	r.QueryCountCalls = append(r.QueryCountCalls, sql)
	if len(r.counts) == 0 {
		return 0, nil
	}
	n := r.counts[0]
	r.counts = r.counts[1:]
	return n, nil
}

func TestEvaluateDataQuality_ReportsEveryAssertionInKeyOrder(t *testing.T) {
	r := &MockRunner{}
	cfg := map[string]string{
		"test_unique":                 "id",
		"test_not_null":               "id, email",
		"test_accepted_values_status": "active, it's",
		"materialized":                "table",
	}
	got := EvaluateDataQuality(context.Background(), r, "users", cfg, "s", "users", nil)

	var labels []string
	for _, res := range got {
		if !res.Passed || res.Err != nil {
			t.Errorf("%+v: want pass", res)
		}
		labels = append(labels, res.Kind+":"+res.Column)
	}
	want := "accepted_values:status,not_null:id,not_null:email,unique:id"
	if strings.Join(labels, ",") != want {
		t.Fatalf("results = %s, want %s", strings.Join(labels, ","), want)
	}
	if len(r.QueryCountCalls) != 4 {
		t.Fatalf("queries = %d, want 4", len(r.QueryCountCalls))
	}
	if !strings.Contains(r.QueryCountCalls[0], "'it''s'") {
		t.Errorf("accepted_values literal not escaped: %s", r.QueryCountCalls[0])
	}
	if strings.Contains(r.QueryCountCalls[3], "_c") || !strings.Contains(r.QueryCountCalls[3], "HAVING COUNT(*) > 1) AS sqlforge_dups") {
		t.Errorf("unique SQL is not portable: %s", r.QueryCountCalls[3])
	}
}

func TestEvaluateDataQuality_FailuresAndErrors(t *testing.T) {
	r := &clickhouseCountRunner{counts: []int{3}}
	got := EvaluateDataQuality(context.Background(), r, "t", map[string]string{
		"test_not_null":            "id",
		"test_accepted_values_x":   `a\'b`,
		"test_unique":              "id); DROP TABLE x; --",
		"test_relationship":        "uid to users.id",
		"test_relationship_broken": "not parseable",
	}, "s", "t", &ExecutionPlan{Environment: &state.Environment{Schema: "s"}, Unchanged: []*model.Asset{{Name: "users", Config: map[string]string{}}}})

	byKind := map[string]QualityResult{}
	for _, res := range got {
		byKind[res.Kind] = res
	}
	if len(got) != 4 {
		t.Fatalf("results = %+v, want 4 (malformed relationship is skipped)", got)
	}
	if res := byKind["accepted_values"]; res.Passed || res.Err == nil || !strings.Contains(res.Err.Error(), "3 records") {
		t.Errorf("accepted_values = %+v, want failure with count 3", res)
	}
	if res := byKind["unique"]; res.Passed || res.Err == nil || !strings.Contains(res.Err.Error(), "unsafe test config") {
		t.Errorf("unique = %+v, want unsafe-config error", res)
	}
	if res := byKind["relationship"]; !res.Passed || res.Detail != "referenced users.id" {
		t.Errorf("relationship = %+v", res)
	}
	for _, sql := range r.QueryCountCalls {
		if strings.Contains(sql, "DROP") {
			t.Fatalf("hostile config reached the warehouse: %s", sql)
		}
	}
	if !strings.Contains(strings.Join(r.QueryCountCalls, "\n"), `x NOT IN ('a\\''b')`) {
		t.Errorf("clickhouse literal not escaped: %v", r.QueryCountCalls)
	}
	if len(r.QueryCountCalls) != 3 {
		t.Errorf("queries = %d, want 3 (unsafe unique never queried)", len(r.QueryCountCalls))
	}
}

func TestEvaluateDataQuality_QueryError(t *testing.T) {
	r := &MockRunner{QueryCountErr: errors.New("boom")}
	got := EvaluateDataQuality(context.Background(), r, "t", map[string]string{"test_not_null": "id"}, "s", "t", nil)
	if len(got) != 1 || got[0].Passed || got[0].Err == nil || got[0].Err.Error() != "boom" {
		t.Fatalf("got %+v", got)
	}
}
