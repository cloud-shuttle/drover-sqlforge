package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"pgregory.net/rapid"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// minimalPlan returns an ExecutionPlan with one unchanged asset that has the
// given custom schema, useful for relationship-check parent resolution tests.
func minimalPlan(envSchema, parentName, customSchema string) *ExecutionPlan {
	return &ExecutionPlan{
		Environment: &state.Environment{Name: "test", Schema: envSchema},
		Unchanged: []*model.Asset{
			{
				Name:   parentName,
				Config: map[string]string{"schema": customSchema},
			},
		},
	}
}

// ─── Matches contract ─────────────────────────────────────────────────────────

func TestMatches(t *testing.T) {
	cases := []struct {
		check QualityCheck
		key   string
		want  bool
	}{
		{notNullCheck{}, "test_not_null", true},
		{notNullCheck{}, "test_unique", false},
		{uniqueCheck{}, "test_unique", true},
		{uniqueCheck{}, "test_not_null", false},
		{acceptedValuesCheck{}, "test_accepted_values_status", true},
		{acceptedValuesCheck{}, "test_not_null", false},
		{relationshipCheck{}, "test_relationship", true},
		{relationshipCheck{}, "test_relationship_custom", true},
		{relationshipCheck{}, "test_not_null", false},
	}
	for _, tc := range cases {
		got := tc.check.Matches(tc.key)
		if got != tc.want {
			t.Errorf("%T.Matches(%q) = %v, want %v", tc.check, tc.key, got, tc.want)
		}
	}
}

// ─── SQL generation contracts ─────────────────────────────────────────────────

func TestNotNullCheck_SingleColumn(t *testing.T) {
	check := notNullCheck{}
	qs := check.Queries("myschema", "orders", "test_not_null", "user_id", nil)
	if len(qs) != 1 {
		t.Fatalf("expected 1 query, got %d", len(qs))
	}
	if !strings.Contains(qs[0].SQL, "user_id IS NULL") {
		t.Errorf("expected IS NULL predicate on user_id: %s", qs[0].SQL)
	}
}

func TestNotNullCheck_MultiColumn(t *testing.T) {
	check := notNullCheck{}
	qs := check.Queries("s", "t", "test_not_null", "user_id, order_date", nil)
	if len(qs) != 2 {
		t.Fatalf("expected 2 queries for 2 columns, got %d", len(qs))
	}
	if !strings.Contains(qs[0].SQL, "user_id") {
		t.Errorf("first query missing user_id: %s", qs[0].SQL)
	}
	if !strings.Contains(qs[1].SQL, "order_date") {
		t.Errorf("second query missing order_date: %s", qs[1].SQL)
	}
}

func TestNotNullCheck_FailMsg(t *testing.T) {
	check := notNullCheck{}
	qs := check.Queries("s", "t", "test_not_null", "email", nil)
	err := qs[0].onFail(7)
	if err == nil {
		t.Fatal("expected non-nil error from onFail")
	}
	msg := err.Error()
	for _, want := range []string{"email", "7", "not_null", "null records"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected %q in error message, got: %s", want, msg)
		}
	}
}

func TestUniqueCheck_SQL(t *testing.T) {
	check := uniqueCheck{}
	qs := check.Queries("s", "t", "test_unique", "order_id", nil)
	if len(qs) != 1 {
		t.Fatalf("expected 1 query, got %d", len(qs))
	}
	sql := qs[0].SQL
	for _, kw := range []string{"GROUP BY", "HAVING", "order_id"} {
		if !strings.Contains(sql, kw) {
			t.Errorf("SQL missing %q: %s", kw, sql)
		}
	}
}

func TestUniqueCheck_FailMsg(t *testing.T) {
	check := uniqueCheck{}
	qs := check.Queries("s", "t", "test_unique", "order_id", nil)
	err := qs[0].onFail(3)
	msg := err.Error()
	for _, want := range []string{"order_id", "3", "unique", "duplicate"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected %q in error message, got: %s", want, msg)
		}
	}
}

func TestAcceptedValuesCheck_SQL(t *testing.T) {
	check := acceptedValuesCheck{}
	qs := check.Queries("s", "t", "test_accepted_values_status", "active, inactive, pending", nil)
	if len(qs) != 1 {
		t.Fatalf("expected 1 query, got %d", len(qs))
	}
	sql := qs[0].SQL
	if !strings.Contains(sql, "NOT IN") {
		t.Errorf("expected NOT IN: %s", sql)
	}
	for _, v := range []string{"'active'", "'inactive'", "'pending'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("expected quoted value %s: %s", v, sql)
		}
	}
}

func TestRelationshipCheck_ToSyntax(t *testing.T) {
	check := relationshipCheck{}
	plan := minimalPlan("sf__test", "stg_users", "staging")
	qs := check.Queries("sf__test", "customer_360", "test_relationship", "user_id to stg_users.user_id", plan)
	if len(qs) != 1 {
		t.Fatalf("expected 1 query, got %d", len(qs))
	}
	sql := qs[0].SQL
	for _, kw := range []string{"IS NOT NULL", "NOT IN", "stg_users"} {
		if !strings.Contains(sql, kw) {
			t.Errorf("SQL missing %q: %s", kw, sql)
		}
	}
}

func TestRelationshipCheck_ArrowSyntax(t *testing.T) {
	check := relationshipCheck{}
	plan := minimalPlan("sf__test", "stg_users", "staging")
	qs := check.Queries("sf__test", "customer_360", "test_relationship", "user_id->stg_users.user_id", plan)
	if len(qs) != 1 {
		t.Fatalf("expected 1 query for arrow syntax, got %d", len(qs))
	}
}

func TestRelationshipCheck_MalformedValue(t *testing.T) {
	check := relationshipCheck{}
	qs := check.Queries("s", "t", "test_relationship", "not valid at all", nil)
	if qs != nil {
		t.Errorf("expected nil for malformed value, got %v", qs)
	}
}

// ─── Full dispatch integration ────────────────────────────────────────────────

func TestRunDataQualityTestsFromConfig_NotNull_Pass(t *testing.T) {
	runner := &MockRunner{QueryCountResult: 0}
	cfg := map[string]string{"test_not_null": "email"}
	if err := RunDataQualityTestsFromConfig(context.Background(), runner, cfg, "s", "t", nil); err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
}

func TestRunDataQualityTestsFromConfig_NotNull_Fail(t *testing.T) {
	runner := &MockRunner{QueryCountResult: 5}
	cfg := map[string]string{"test_not_null": "email"}
	err := RunDataQualityTestsFromConfig(context.Background(), runner, cfg, "s", "t", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not_null") {
		t.Errorf("expected 'not_null' in error: %v", err)
	}
}

func TestRunDataQualityTestsFromConfig_Unique_Fail(t *testing.T) {
	runner := &MockRunner{QueryCountResult: 2}
	cfg := map[string]string{"test_unique": "order_id"}
	err := RunDataQualityTestsFromConfig(context.Background(), runner, cfg, "s", "t", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unique") {
		t.Errorf("expected 'unique' in error: %v", err)
	}
}

func TestRunDataQualityTestsFromConfig_AcceptedValues_Fail(t *testing.T) {
	runner := &MockRunner{QueryCountResult: 1}
	cfg := map[string]string{"test_accepted_values_status": "active, inactive"}
	err := RunDataQualityTestsFromConfig(context.Background(), runner, cfg, "s", "t", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "accepted values") {
		t.Errorf("expected 'accepted values' in error: %v", err)
	}
}

func TestRunDataQualityTestsFromConfig_UnknownKey_Ignored(t *testing.T) {
	// Config keys that don't match any check should be silently skipped.
	runner := &MockRunner{QueryCountResult: 999}
	cfg := map[string]string{"materialized": "table", "dialect": "clickhouse"}
	if err := RunDataQualityTestsFromConfig(context.Background(), runner, cfg, "s", "t", nil); err != nil {
		t.Fatalf("expected unknown keys to be ignored, got: %v", err)
	}
	if len(runner.QueryCountCalls) != 0 {
		t.Errorf("expected no queries for non-test keys, got %d", len(runner.QueryCountCalls))
	}
}

// ─── Property-based tests ─────────────────────────────────────────────────────

func TestProp_NotNull_SQLContainsColumn(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		col := rapid.StringMatching(`[a-z][a-z0-9_]{0,15}`).Draw(t, "col")
		qs := notNullCheck{}.Queries("s", "t", "test_not_null", col, nil)
		if len(qs) != 1 {
			t.Fatalf("expected 1 query, got %d", len(qs))
		}
		if !strings.Contains(qs[0].SQL, col) {
			t.Fatalf("SQL missing column %q: %s", col, qs[0].SQL)
		}
		if !strings.Contains(qs[0].SQL, "IS NULL") {
			t.Fatalf("SQL missing IS NULL: %s", qs[0].SQL)
		}
	})
}

func TestProp_Unique_GroupByHavingInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		col := rapid.StringMatching(`[a-z][a-z0-9_]{0,15}`).Draw(t, "col")
		qs := uniqueCheck{}.Queries("s", "t", "test_unique", col, nil)
		if len(qs) != 1 {
			t.Fatalf("expected 1 query, got %d", len(qs))
		}
		for _, kw := range []string{"GROUP BY", "HAVING", col} {
			if !strings.Contains(qs[0].SQL, kw) {
				t.Fatalf("SQL missing %q: %s", kw, qs[0].SQL)
			}
		}
	})
}

func TestProp_AcceptedValues_AllValuesQuoted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		vals := rapid.SliceOfN(rapid.StringMatching(`[a-z]{1,10}`), 1, 8).Draw(t, "vals")
		colName := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "col")
		joined := strings.Join(vals, ", ")

		qs := acceptedValuesCheck{}.Queries("s", "t", "test_accepted_values_"+colName, joined, nil)
		if len(qs) != 1 {
			t.Fatalf("expected 1 query, got %d", len(qs))
		}
		for _, v := range vals {
			if !strings.Contains(qs[0].SQL, "'"+v+"'") {
				t.Fatalf("SQL missing quoted value %q: %s", v, qs[0].SQL)
			}
		}
	})
}

func TestProp_SplitCSV_PreservesValues(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		parts := rapid.SliceOfN(
			rapid.StringMatching(`[a-z][a-z0-9_]{0,10}`), 1, 10,
		).Draw(t, "parts")
		joined := strings.Join(parts, " , ")
		got := splitCSV(joined)
		if len(got) != len(parts) {
			t.Fatalf("expected %d parts, got %d (input: %q)", len(parts), len(got), joined)
		}
		for i := range parts {
			if got[i] != parts[i] {
				t.Fatalf("part[%d]: want %q, got %q", i, parts[i], got[i])
			}
		}
	})
}
