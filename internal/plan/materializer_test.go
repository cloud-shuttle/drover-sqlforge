package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/state"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func makeMaterializer(t *testing.T, execPlan *ExecutionPlan, runner *MockRunner) *ModelMaterializer {
	t.Helper()
	tmpDir := t.TempDir()
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("state.NewManager: %v", err)
	}
	vMgr := newMockVMgr(runner, stateMgr)
	allModels := allModelNames(execPlan)
	return newModelMaterializer(execPlan, allModels, nil, vMgr, stateMgr, nil, len(execPlan.ChangedModels))
}

func allModelNames(p *ExecutionPlan) map[string]bool {
	m := make(map[string]bool)
	for _, a := range append(p.ChangedModels, append(p.Impacted, p.Unchanged...)...) {
		m[a.Name] = true
	}
	return m
}

func simpleAsset(name, mat string) *model.Asset {
	return &model.Asset{
		Name:   name,
		SQL:    "SELECT 1",
		Config: map[string]string{"materialized": mat},
	}
}

func singleModelPlan(a *model.Asset) *ExecutionPlan {
	return &ExecutionPlan{
		Environment:   &state.Environment{Name: "test", Schema: "sf__test"},
		ChangedModels: []*model.Asset{a},
	}
}

// ─── Materialization routing ──────────────────────────────────────────────────

func TestMaterializer_View(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("my_view", "view")
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !strings.HasPrefix(runner.LastDDL, "view my_view") {
		t.Errorf("expected view DDL, got: %s", runner.LastDDL)
	}
}

func TestMaterializer_Table(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("my_table", "table")
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !strings.HasPrefix(runner.LastDDL, "table my_table") {
		t.Errorf("expected table DDL, got: %s", runner.LastDDL)
	}
}

func TestMaterializer_MaterializedView(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("mv", "materialized_view")
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !strings.HasPrefix(runner.LastDDL, "mview mv") {
		t.Errorf("expected mview DDL, got: %s", runner.LastDDL)
	}
}

func TestMaterializer_KafkaStreaming(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("events_kafka", "kafka")
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if runner.LastDDL != "stream events_kafka kafka" {
		t.Errorf("expected kafka stream DDL, got: %s", runner.LastDDL)
	}
}

func TestMaterializer_NATSStreaming(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("events_nats", "nats")
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if runner.LastDDL != "stream events_nats nats" {
		t.Errorf("expected nats stream DDL, got: %s", runner.LastDDL)
	}
}

// TestMaterializer_DefaultsToView verifies that a model with no "materialized"
// config key is treated as a view — the documented default.
func TestMaterializer_DefaultsToView(t *testing.T) {
	runner := &MockRunner{}
	a := &model.Asset{
		Name:   "no_mat_type",
		SQL:    "SELECT 1",
		Config: map[string]string{}, // no "materialized" key
	}
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !strings.HasPrefix(runner.LastDDL, "view no_mat_type") {
		t.Errorf("expected view DDL for missing materialization, got: %s", runner.LastDDL)
	}
}

// ─── State persistence ────────────────────────────────────────────────────────

// TestMaterializer_FingerprintSavedAfterApply verifies that after a successful
// Apply, the model state is persisted so subsequent plan runs see it as unchanged.
func TestMaterializer_FingerprintSavedAfterApply(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("orders", "view")
	execPlan := singleModelPlan(a)

	tmpDir := t.TempDir()
	stateMgr, _ := state.NewManager(tmpDir)
	vMgr := newMockVMgr(runner, stateMgr)
	mat := newModelMaterializer(execPlan, allModelNames(execPlan), nil, vMgr, stateMgr, nil, 1)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Reload state — fingerprint must be present.
	stateMgr2, _ := state.NewManager(tmpDir)
	saved, err := stateMgr2.Store.GetModelState("orders", "test")
	if err != nil {
		t.Fatalf("expected saved model state, got error: %v", err)
	}
	if saved.MaterializedAs != "view" {
		t.Errorf("expected MaterializedAs=view, got %q", saved.MaterializedAs)
	}
	if saved.Environment != "test" {
		t.Errorf("expected Environment=test, got %q", saved.Environment)
	}
}

// ─── Event emission ───────────────────────────────────────────────────────────

func TestMaterializer_EmitsStartAndSuccessEvents(t *testing.T) {
	runner := &MockRunner{}
	a := simpleAsset("events_model", "view")
	execPlan := singleModelPlan(a)

	tmpDir := t.TempDir()
	stateMgr, _ := state.NewManager(tmpDir)
	vMgr := newMockVMgr(runner, stateMgr)

	ch := make(chan ApplyEvent, 10)
	mat := newModelMaterializer(execPlan, allModelNames(execPlan), nil, vMgr, stateMgr, ch, 1)

	if err := mat.Apply(context.Background(), a); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	close(ch)

	var events []ApplyEvent
	for e := range ch {
		events = append(events, e)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events (START + SUCCESS), got %d", len(events))
	}
	if events[0].Type != EventStart || events[0].ModelName != "events_model" {
		t.Errorf("expected START event for events_model, got %+v", events[0])
	}
	if events[1].Type != EventSuccess {
		t.Errorf("expected SUCCESS event, got %+v", events[1])
	}
}

func TestMaterializer_EmitsErrorEventOnExecFailure(t *testing.T) {
	runner := &MockRunner{ExecErr: errMockExec}
	a := simpleAsset("bad_model", "view")
	execPlan := singleModelPlan(a)

	tmpDir := t.TempDir()
	stateMgr, _ := state.NewManager(tmpDir)
	vMgr := newMockVMgr(runner, stateMgr)

	ch := make(chan ApplyEvent, 10)
	mat := newModelMaterializer(execPlan, allModelNames(execPlan), nil, vMgr, stateMgr, ch, 1)

	err := mat.Apply(context.Background(), a)
	if err == nil {
		t.Fatal("expected error from exec failure")
	}
	close(ch)

	var gotError bool
	for e := range ch {
		if e.Type == EventError {
			gotError = true
		}
	}
	if !gotError {
		t.Error("expected ERROR event to be emitted")
	}
}

// ─── Dependency resolution ────────────────────────────────────────────────────

// TestMaterializer_DependencyResolved verifies that when a model has a
// dependency in the plan, the compiled SQL has the dep substituted to its
// fully-qualified name.
func TestMaterializer_DependencyResolved(t *testing.T) {
	runner := &MockRunner{}

	upstream := &model.Asset{
		Name:   "stg_orders",
		SQL:    "SELECT 1",
		Config: map[string]string{"materialized": "view"},
	}
	downstream := &model.Asset{
		Name:         "orders_enriched",
		SQL:          "SELECT * FROM stg_orders",
		Config:       map[string]string{"materialized": "view"},
		Dependencies: []string{"stg_orders"},
	}
	execPlan := &ExecutionPlan{
		Environment:   &state.Environment{Name: "test", Schema: "sf__test"},
		ChangedModels: []*model.Asset{upstream, downstream},
	}

	tmpDir := t.TempDir()
	stateMgr, _ := state.NewManager(tmpDir)
	vMgr := newMockVMgr(runner, stateMgr)
	allModels := allModelNames(execPlan)
	mat := newModelMaterializer(execPlan, allModels, nil, vMgr, stateMgr, nil, 2)

	// Apply the downstream model — it depends on stg_orders
	if err := mat.Apply(context.Background(), downstream); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// The SQL passed to the DDL generator must contain the FQN of stg_orders.
	if !strings.Contains(runner.LastSQL, "sf__test.stg_orders") {
		t.Errorf("expected FQN sf__test.stg_orders in compiled SQL, got: %s", runner.LastSQL)
	}
}

// ─── Data quality test integration ───────────────────────────────────────────

// TestMaterializer_QualityFailure propagates the DQ error and emits ERROR event.
func TestMaterializer_QualityFailure(t *testing.T) {
	// QueryCount returns 1 — simulates a not_null violation.
	runner := &MockRunner{QueryCountResult: 1}
	a := &model.Asset{
		Name: "orders",
		SQL:  "SELECT 1",
		Config: map[string]string{
			"materialized": "view",
			"test_not_null": "customer_id",
		},
	}
	mat := makeMaterializer(t, singleModelPlan(a), runner)

	err := mat.Apply(context.Background(), a)
	if err == nil {
		t.Fatal("expected data quality error, got nil")
	}
	if !strings.Contains(err.Error(), "not_null") {
		t.Errorf("expected 'not_null' in error, got: %v", err)
	}
}
