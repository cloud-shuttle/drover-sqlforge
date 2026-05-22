package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

type MockRunner struct {
	LastDDL string
}

func (m *MockRunner) Exec(ctx context.Context, sql string) error {
	m.LastDDL = sql
	return nil
}
func (m *MockRunner) CreateSchemaDDL(schema string) string { return "schema " + schema }
func (m *MockRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return "table " + table
}
func (m *MockRunner) CreateViewDDL(schema, table, selectSQL string) string {
	return "view " + table
}
func (m *MockRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return "mview " + table
}
func (m *MockRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return "stream " + table + " " + config["_materialization_type"]
}
func (m *MockRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	return m.LastDDL != "", nil // Return true if it was run once in the test
}
func (m *MockRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	return "merge " + table
}
func (m *MockRunner) QueryCount(ctx context.Context, sql string) (int, error) {
	return 0, nil
}
func (m *MockRunner) Name() string { return "duckdb" }

func (m *MockRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	return nil, nil
}

func TestApplyPlanRouting(t *testing.T) {
	tmpDir := t.TempDir()
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	runner := &MockRunner{}
	vMgr := virtual.NewManager(runner, stateMgr)

	ctx := context.Background()

	p := &ExecutionPlan{
		Environment: &state.Environment{
			Name:    "test_env",
			Schema:  "test_schema",
			BaseEnv: "prod",
		},
		ChangedModels: []*model.Asset{
			{
				Name: "events_kafka",
				Config: map[string]string{
					"materialized": "kafka",
				},
			},
		},
	}

	err = ApplyPlan(ctx, p, stateMgr, vMgr, nil)
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	if runner.LastDDL != "stream events_kafka kafka" {
		t.Errorf("Expected stream routing for kafka, got %s", runner.LastDDL)
	}

	// Test NATS
	p.ChangedModels[0].Name = "events_nats"
	p.ChangedModels[0].Config["materialized"] = "nats"
	runner.LastDDL = ""

	err = ApplyPlan(ctx, p, stateMgr, vMgr, nil)
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	if runner.LastDDL != "stream events_nats nats" {
		t.Errorf("Expected stream routing for nats, got %s", runner.LastDDL)
	}

	// Test Materialized View
	p.ChangedModels[0].Name = "events_mview"
	p.ChangedModels[0].Config["materialized"] = "materialized_view"
	runner.LastDDL = ""

	err = ApplyPlan(ctx, p, stateMgr, vMgr, nil)
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	if runner.LastDDL != "mview events_mview" && !strings.HasPrefix(runner.LastDDL, "mview events_mview") {
		t.Errorf("Expected mview routing, got %s", runner.LastDDL)
	}
}
