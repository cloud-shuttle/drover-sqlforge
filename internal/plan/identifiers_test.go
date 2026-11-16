package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
	"github.com/cloud-shuttle/drover-sqlforge/internal/virtual"
)

// recordingRunner wraps MockRunner and records every Exec.
type recordingRunner struct {
	MockRunner
	execs []string
}

func (r *recordingRunner) Exec(ctx context.Context, sql string) error {
	r.execs = append(r.execs, sql)
	return r.MockRunner.Exec(ctx, sql)
}

func identPlan(schema string, a *model.Asset) *ExecutionPlan {
	return &ExecutionPlan{
		Environment:   &state.Environment{Name: "dev", Schema: schema, BaseEnv: "prod"},
		ChangedModels: []*model.Asset{a},
	}
}

func TestValidateApplyTargets(t *testing.T) {
	good := &model.Asset{Name: "orders", Config: map[string]string{
		"materialized":                  "incremental",
		"unique_key":                    "id, tenant_id",
		"updated_at":                    "updated_at",
		"schema":                        "marts",
		"alias":                         "orders_v2",
		"database":                      "analytics",
		"test_not_null":                 "id, email",
		"test_unique":                   "id",
		"test_accepted_values_status":   "active, it's",
		"test_relationship":             "user_id to users.id",
		"test_relationship_malformed_x": "not parseable",
	}}
	if err := ValidateApplyTargets(identPlan("sqlforge__dev", good)); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}

	bad := map[string]*model.Asset{
		"env schema":       nil,
		"model name":       {Name: "orders;DROP TABLE x", Config: map[string]string{}},
		"alias":            {Name: "o", Config: map[string]string{"alias": "o; DROP TABLE x"}},
		"schema suffix":    {Name: "o", Config: map[string]string{"schema": "marts.x"}},
		"database":         {Name: "o", Config: map[string]string{"database": "db--"}},
		"unique_key":       {Name: "o", Config: map[string]string{"unique_key": "id) OR 1=1 --"}},
		"updated_at":       {Name: "o", Config: map[string]string{"updated_at": "ts; DROP"}},
		"not_null col":     {Name: "o", Config: map[string]string{"test_not_null": "id, 1=1 OR x"}},
		"unique col":       {Name: "o", Config: map[string]string{"test_unique": "id)"}},
		"accepted col":     {Name: "o", Config: map[string]string{"test_accepted_values_a b": "x"}},
		"relationship col": {Name: "o", Config: map[string]string{"test_relationship": "id to users.id) OR (1=1"}},
	}
	for name, a := range bad {
		p := identPlan("sqlforge__dev", a)
		if a == nil {
			p = identPlan("sqlforge__dev; DROP SCHEMA prod", good)
		}
		if err := ValidateApplyTargets(p); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestApplyPlan_RejectsUnsafeIdentifiersBeforeAnySQL(t *testing.T) {
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	vMgr := virtual.NewManager(runner, stateMgr)
	p := identPlan("sqlforge__dev", &model.Asset{Name: "o", SQL: "SELECT 1", Config: map[string]string{"alias": "o; DROP TABLE users"}})

	err = ApplyPlan(context.Background(), p, stateMgr, vMgr, nil, nil, 1)
	if err == nil || !strings.Contains(err.Error(), "invalid table name") {
		t.Fatalf("err = %v", err)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("SQL executed before validation: %q", runner.execs)
	}
}

// emptyDDLRunner simulates a plugin RPC failure (GRPCClient returns "").
type emptyDDLRunner struct{ recordingRunner }

func (r *emptyDDLRunner) CreateViewDDL(schema, table, selectSQL string) string { return "" }

// errorDDLRunner simulates a generator that reports failure as a comment.
type errorDDLRunner struct{ recordingRunner }

func (r *errorDDLRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return "-- error: boom"
}

func TestMaterializer_EmptyOrErrorDDLFails(t *testing.T) {
	for name, runner := range map[string]interface {
		virtual.Runner
	}{
		"empty":         &emptyDDLRunner{},
		"error comment": &errorDDLRunner{},
	} {
		stateMgr, _ := state.NewManager(t.TempDir())
		vMgr := virtual.NewManager(runner, stateMgr)
		p := identPlan("sqlforge__dev", simpleAsset("orders", "view"))
		m := newModelMaterializer(p, allModelNames(p), nil, vMgr, stateMgr, nil, 1)
		if err := m.Apply(context.Background(), p.ChangedModels[0]); err == nil {
			t.Errorf("%s: want error", name)
		}
		var execs []string
		switch r := runner.(type) {
		case *emptyDDLRunner:
			execs = r.execs
		case *errorDDLRunner:
			execs = r.execs
		}
		if len(execs) != 0 {
			t.Errorf("%s: executed %q", name, execs)
		}
	}

	// "-- error:" from an incremental builder (invalid unique_key) must fail.
	runner := &MockRunner{}
	a := simpleAsset("orders", "incremental")
	a.Config["unique_key"] = "id"
	p := identPlan("sqlforge__dev", a)
	m := makeMaterializer(t, p, runner)
	a.Config["unique_key"] = "id; DROP TABLE x" // bypass the plan-level gate
	if err := m.Apply(context.Background(), a); err == nil {
		t.Fatal("want error for unsafe unique_key")
	}
}

func TestQualityChecks_InvalidColumnsReturnError(t *testing.T) {
	runner := &MockRunner{}
	cases := map[string]string{
		"test_not_null":               "id, x OR 1=1",
		"test_unique":                 "id)",
		"test_accepted_values_a;b":    "x",
		"test_relationship":           "id to users.id) --",
		"test_accepted_values_status": "", // valid column, no values: no error expected below
	}
	for k, v := range cases {
		err := RunDataQualityTestsFromConfig(context.Background(), runner, map[string]string{k: v}, "s", "t", nil)
		if k == "test_accepted_values_status" {
			continue
		}
		if err == nil {
			t.Errorf("%s=%q: want error", k, v)
		}
	}
	for _, q := range runner.QueryCountCalls {
		if strings.Contains(q, "OR 1=1") || strings.Contains(q, ") --") {
			t.Fatalf("unsafe query executed: %s", q)
		}
	}
}

func TestAcceptedValues_EscapesLiteralsPerDialect(t *testing.T) {
	qs := acceptedValuesCheck{}.Queries("duckdb", "s", "t", "test_accepted_values_status", `it's, a\`, nil)
	if len(qs) != 1 || !strings.Contains(qs[0].SQL, `('it''s', 'a\')`) {
		t.Fatalf("duckdb SQL = %v", qs)
	}
	qs = acceptedValuesCheck{}.Queries("clickhouse", "s", "t", "test_accepted_values_status", `x\' OR 1=1 --`, nil)
	if len(qs) != 1 || !strings.Contains(qs[0].SQL, `('x\\'' OR 1=1 --')`) {
		t.Fatalf("clickhouse SQL = %v", qs)
	}
}
