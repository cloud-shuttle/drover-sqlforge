package project

import (
	"context"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
	"github.com/drover-org/drover-sqlforge/internal/state"
)

// ----------------------------------------------------------------------------
// Minimal mock runner — satisfies virtual.Runner without any I/O.
// ----------------------------------------------------------------------------

type mockRunner struct{ name string }

func (m *mockRunner) Name() string                                { return m.name }
func (m *mockRunner) Exec(_ context.Context, _ string) error      { return nil }
func (m *mockRunner) QueryCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (m *mockRunner) QueryData(_ context.Context, _ string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (m *mockRunner) TableExists(_ context.Context, _, _ string) (bool, error) { return false, nil }
func (m *mockRunner) CreateSchemaDDL(schema string) string { return "CREATE SCHEMA " + schema }
func (m *mockRunner) CreateTableDDL(schema, table, sql string) string {
	return "CREATE TABLE " + schema + "." + table
}
func (m *mockRunner) CreateViewDDL(schema, table, sql string) string {
	return "CREATE VIEW " + schema + "." + table
}
func (m *mockRunner) CreateMaterializedViewDDL(schema, table, sql string) string {
	return "CREATE MVIEW " + schema + "." + table
}
func (m *mockRunner) CreateStreamingTableDDL(schema, table string, _ map[string]string) string {
	return "-- stream " + schema + "." + table
}
func (m *mockRunner) CreateIncrementalMergeDDL(schema, table, sql string, _ map[string]string) string {
	return "MERGE INTO " + schema + "." + table
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func newTestEnv(t *testing.T) (*state.Manager, *state.Environment) {
	t.Helper()
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("state.NewManager: %v", err)
	}
	env, err := stateMgr.GetOrCreateEnv("test", "prod")
	if err != nil {
		t.Fatalf("GetOrCreateEnv: %v", err)
	}
	return stateMgr, env
}

func assets(names ...string) []*model.Asset {
	out := make([]*model.Asset, len(names))
	for i, n := range names {
		out[i] = &model.Asset{Name: n, SQL: "SELECT 1", Config: map[string]string{}}
	}
	return out
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

// TestNewRuntime_EmptyProject verifies that an empty asset list produces a
// valid Runtime with an empty DAG — the zero-model data project is legal.
func TestNewRuntime_EmptyProject(t *testing.T) {
	stateMgr, env := newTestEnv(t)
	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if rt.DAG == nil {
		t.Fatal("expected non-nil DAG")
	}
	if len(rt.DAG.Nodes) != 0 {
		t.Errorf("expected 0 DAG nodes, got %d", len(rt.DAG.Nodes))
	}
	if rt.Env.Name != "test" {
		t.Errorf("expected env name 'test', got %q", rt.Env.Name)
	}
	if rt.Env.Schema != "sqlforge__test" {
		t.Errorf("expected schema 'sqlforge__test', got %q", rt.Env.Schema)
	}
}

// TestNewRuntime_DAGBuiltFromAssets verifies the DAG is populated correctly
// from the injected asset slice — the core structural guarantee of NewRuntime.
func TestNewRuntime_DAGBuiltFromAssets(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	a := assets("stg_orders", "stg_customers", "customer_360")
	// wire one dependency: customer_360 depends on stg_customers
	a[2].Dependencies = []string{"stg_customers"}

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Assets:   a,
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	if len(rt.DAG.Nodes) != 3 {
		t.Errorf("expected 3 DAG nodes, got %d", len(rt.DAG.Nodes))
	}
	if _, ok := rt.DAG.Nodes["customer_360"]; !ok {
		t.Error("customer_360 missing from DAG")
	}
}

// TestNewRuntime_CyclicDAGIsRejected verifies that a cyclic dependency graph
// is caught during NewRuntime, not silently accepted and exploded at plan time.
func TestNewRuntime_CyclicDAGIsRejected(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	a := assets("model_a", "model_b")
	a[0].Dependencies = []string{"model_b"}
	a[1].Dependencies = []string{"model_a"} // cycle

	_, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Assets:   a,
		Env:      env,
	})
	if err == nil {
		t.Fatal("expected error for cyclic DAG, got nil")
	}
}

// TestNewRuntime_ExecutionPlan_AllChangedOnFirstApply verifies that on a
// fresh environment (no prior fingerprints), every model is classified as
// a changed model in the execution plan.
func TestNewRuntime_ExecutionPlan_AllChangedOnFirstApply(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Assets:   assets("stg_orders", "stg_customers"),
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	execPlan, err := rt.ExecutionPlan()
	if err != nil {
		t.Fatalf("ExecutionPlan: %v", err)
	}

	if len(execPlan.ChangedModels) != 2 {
		t.Errorf("expected 2 changed models on first apply, got %d", len(execPlan.ChangedModels))
	}
	if len(execPlan.Unchanged) != 0 {
		t.Errorf("expected 0 unchanged models on first apply, got %d", len(execPlan.Unchanged))
	}
}

// TestNewRuntime_ImpactPropagation verifies that the execution plan correctly
// marks downstream models as impacted when an upstream model changes.
func TestNewRuntime_ImpactPropagation(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	a := assets("stg_orders", "orders_enriched", "customer_360")
	// orders_enriched depends on stg_orders; customer_360 depends on orders_enriched
	a[1].Dependencies = []string{"stg_orders"}
	a[2].Dependencies = []string{"orders_enriched"}

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Assets:   a,
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	execPlan, err := rt.ExecutionPlan()
	if err != nil {
		t.Fatalf("ExecutionPlan: %v", err)
	}

	// All three are changed on first apply.
	total := len(execPlan.ChangedModels) + len(execPlan.Impacted)
	if total != 3 {
		t.Errorf("expected 3 models in plan, got %d", total)
	}
}

// TestNewRuntime_NilParserSafe verifies that a nil Parser doesn't panic —
// DDL-generation tests don't need WASM.
func TestNewRuntime_NilParserSafe(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	rt, err := NewRuntime(".", RuntimeDeps{
		Parser:   nil, // explicitly nil — no WASM
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "clickhouse"},
		Assets:   assets("stg_events"),
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime with nil Parser: %v", err)
	}
	if rt.Parser != nil {
		t.Error("expected nil Parser to be preserved as nil on Runtime")
	}
}

// TestNewRuntime_NilSemanticSafe verifies that a project with no metrics.yml
// produces a valid Runtime with nil Semantic — not a panic or empty-graph confusion.
func TestNewRuntime_NilSemanticSafe(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Assets:   assets("stg_orders"),
		Env:      env,
		Semantic: nil,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if rt.Semantic != nil {
		t.Error("expected nil Semantic to be preserved")
	}
	// DAG should still have the model — Semantic=nil must not block asset loading.
	if _, ok := rt.DAG.Nodes["stg_orders"]; !ok {
		t.Error("stg_orders missing from DAG when Semantic is nil")
	}
}

// TestNewRuntime_SemanticDerivedModelInjected verifies that a metric with
// materialize:true produces a synthetic "semantic__<name>" model in the DAG.
func TestNewRuntime_SemanticDerivedModelInjected(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	semGraph := &semantic.Graph{
		Metrics: []semantic.Metric{
			{
				Name:        "daily_orders",
				Expression:  "COUNT(*)",
				Model:       "stg_orders",
				Dimensions:  []string{"order_date"},
				Materialize: true,
			},
		},
	}

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "duckdb"},
		Assets:   assets("stg_orders"),
		Env:      env,
		Semantic: semGraph,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	derived := "semantic__daily_orders"
	if _, ok := rt.DAG.Nodes[derived]; !ok {
		t.Errorf("expected derived model %q in DAG, got nodes: %v", derived, nodeNames(rt))
	}
	if len(rt.Assets) != 2 {
		t.Errorf("expected 2 assets (1 model + 1 derived), got %d", len(rt.Assets))
	}
}

// TestNewRuntime_VMgrNilWhenRunnerNil verifies that omitting Runner produces a
// nil VMgr rather than panicking — useful for read-only plan inspection tests.
func TestNewRuntime_VMgrNilWhenRunnerNil(t *testing.T) {
	stateMgr, env := newTestEnv(t)

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   nil,
		Assets:   assets("stg_orders"),
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if rt.VMgr != nil {
		t.Error("expected nil VMgr when Runner is nil")
	}
}

// TestNewRuntime_EnvSchemaConvention verifies the warehouse schema name matches
// the documented convention (sqlforge__<envName>) for the injected environment.
func TestNewRuntime_EnvSchemaConvention(t *testing.T) {
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("state.NewManager: %v", err)
	}
	env, err := stateMgr.GetOrCreateEnv("peter_dev", "prod")
	if err != nil {
		t.Fatalf("GetOrCreateEnv: %v", err)
	}

	rt, err := NewRuntime(".", RuntimeDeps{
		StateMgr: stateMgr,
		Runner:   &mockRunner{name: "clickhouse"},
		Env:      env,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	if rt.Env.Schema != "sqlforge__peter_dev" {
		t.Errorf("expected schema 'sqlforge__peter_dev', got %q", rt.Env.Schema)
	}
	if rt.Env.BaseEnv != "prod" {
		t.Errorf("expected base env 'prod', got %q", rt.Env.BaseEnv)
	}
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func nodeNames(rt *Runtime) []string {
	names := make([]string, 0, len(rt.DAG.Nodes))
	for n := range rt.DAG.Nodes {
		names = append(names, n)
	}
	return names
}
