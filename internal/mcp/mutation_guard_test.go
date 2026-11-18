package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/graph"
	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
	"github.com/cloud-shuttle/drover-sqlforge/internal/plan"
	"github.com/cloud-shuttle/drover-sqlforge/internal/project"
	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlguard"
	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
	"github.com/cloud-shuttle/drover-sqlforge/internal/virtual"
)

// countingRunner records Exec calls. plan_change must not reach the warehouse.
type countingRunner struct {
	virtual.Runner
	calls int
	sqls  []string
}

func (c *countingRunner) Exec(ctx context.Context, sql string) error {
	c.calls++
	c.sqls = append(c.sqls, sql)
	return c.Runner.Exec(ctx, sql)
}

func guardTestRuntime() *project.Runtime {
	dag := graph.NewDAG()
	dag.AddAsset(&model.Asset{Name: "orders", SQL: "SELECT 1 AS id", Config: map[string]string{}})
	return &project.Runtime{DAG: dag}
}

func TestPlanChange_RejectsUnsafeSQLBeforeStoringPlan(t *testing.T) {
	unsafe := []string{
		"SELECT 1; DROP SCHEMA sqlforge__prod CASCADE",
		"DROP TABLE orders",
		"WITH d AS (DELETE FROM orders RETURNING *) SELECT * FROM d",
		"SELECT * FROM read_csv('/etc/passwd')",
		"SELECT 'a\\'; DROP TABLE x; --'",
	}
	for _, sql := range unsafe {
		store := NewPlanStore()
		reg := NewRegistry()
		reg.InitializeCoreTools(guardTestRuntime(), store)
		tool, _ := reg.Get("plan_change")

		params, _ := json.Marshal(map[string]string{"model_name": "orders", "proposed_sql": sql})
		_, err := tool.Handler(context.Background(), params)
		var ge *sqlguard.Error
		if !errors.As(err, &ge) {
			t.Errorf("plan_change(%q) err = %v, want sqlguard rejection", sql, err)
		}
		if n := len(store.plans); n != 0 {
			t.Errorf("plan_change(%q) stored %d plans", sql, n)
		}
	}
}

func TestPlanChange_MultiStatementDoesNotExecute(t *testing.T) {
	inner, err := virtual.NewRunner("postgres", "stub")
	if err != nil {
		t.Fatal(err)
	}
	counter := &countingRunner{Runner: inner}
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rt := guardTestRuntime()
	rt.StateMgr = stateMgr
	rt.VMgr = virtual.NewManager(counter, stateMgr)

	store := NewPlanStore()
	reg := NewRegistry()
	reg.InitializeCoreTools(rt, store)
	tool, _ := reg.Get("plan_change")
	sql := "SELECT 1; DROP SCHEMA sqlforge__prod CASCADE"
	params, _ := json.Marshal(map[string]string{"model_name": "orders", "proposed_sql": sql})
	_, err = tool.Handler(context.Background(), params)
	var ge *sqlguard.Error
	if !errors.As(err, &ge) {
		t.Fatalf("err = %v, want sqlguard rejection", err)
	}
	if len(store.plans) != 0 {
		t.Fatalf("stored %d plans", len(store.plans))
	}
	if counter.calls != 0 {
		t.Fatalf("executed %d statements: %q", counter.calls, counter.sqls)
	}
}

func TestApplyChange_RejectsPlanWithoutProposal(t *testing.T) {
	store := NewPlanStore()
	id, _ := store.Put(&plan.ExecutionPlan{ChangedModels: []*model.Asset{{Name: "orders", SQL: "SELECT 1"}}})
	_, err := applyChange(context.Background(), guardTestRuntime(), store, id)
	if err == nil || !strings.Contains(err.Error(), "no recorded proposal") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := store.GetPending(id); ok {
		t.Fatal("rejected plan should be discarded")
	}
}

func TestApplyChange_RejectsTamperedPlan(t *testing.T) {
	store := NewPlanStore()
	p := &plan.ExecutionPlan{ChangedModels: []*model.Asset{{Name: "orders", SQL: "SELECT 1; DROP TABLE x"}}}
	id, _ := store.PutProposal(p, "orders", "SELECT 1")
	_, err := applyChange(context.Background(), guardTestRuntime(), store, id)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyChange_RevalidatesStoredSQL(t *testing.T) {
	store := NewPlanStore()
	bad := "SELECT 1; DROP TABLE x"
	p := &plan.ExecutionPlan{ChangedModels: []*model.Asset{{Name: "orders", SQL: bad}}}
	id, _ := store.PutProposal(p, "orders", bad)
	_, err := applyChange(context.Background(), guardTestRuntime(), store, id)
	var ge *sqlguard.Error
	if !errors.As(err, &ge) {
		t.Fatalf("err = %v, want sqlguard rejection", err)
	}
}

func TestRevalidateProposal_AcceptsMatchingSafePlan(t *testing.T) {
	sql := "SELECT 2 AS id"
	p := &PendingPlan{
		Plan:        &plan.ExecutionPlan{ChangedModels: []*model.Asset{{Name: "orders", SQL: sql}}},
		ModelName:   "orders",
		ProposedSQL: sql,
	}
	if err := revalidateProposal(p); err != nil {
		t.Fatalf("err = %v", err)
	}
}
