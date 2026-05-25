package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

// errMockExec is a sentinel error returned by MockRunner.Exec when ExecErr is set.
var errMockExec = errors.New("mock exec error")

type MockRunner struct {
	LastDDL          string
	LastSQL          string
	ExecErr          error
	QueryCountResult int
	QueryCountErr    error
	QueryCountCalls  []string
}

func (m *MockRunner) Exec(ctx context.Context, sql string) error {
	if m.ExecErr != nil {
		return m.ExecErr
	}
	m.LastDDL = sql
	return nil
}
func (m *MockRunner) CreateSchemaDDL(schema string) string { return "schema " + schema }
func (m *MockRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return "table " + table
}
func (m *MockRunner) CreateViewDDL(schema, table, selectSQL string) string {
	m.LastSQL = selectSQL
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
	m.QueryCountCalls = append(m.QueryCountCalls, sql)
	return m.QueryCountResult, m.QueryCountErr
}
func (m *MockRunner) Name() string { return "duckdb" }

func (m *MockRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	return nil, nil
}

// newMockVMgr builds a virtual.Manager backed by the given MockRunner.
// Shared by apply_test.go and materializer_test.go.
func newMockVMgr(runner virtual.Runner, stateMgr *state.Manager) *virtual.Manager {
	return virtual.NewManager(runner, stateMgr)
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

	err = ApplyPlan(ctx, p, stateMgr, vMgr, nil, nil, 4)
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

	err = ApplyPlan(ctx, p, stateMgr, vMgr, nil, nil, 4)
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

	err = ApplyPlan(ctx, p, stateMgr, vMgr, nil, nil, 4)
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	if runner.LastDDL != "mview events_mview" && !strings.HasPrefix(runner.LastDDL, "mview events_mview") {
		t.Errorf("Expected mview routing, got %s", runner.LastDDL)
	}
}

func TestRunDataQualityTests_Relationship(t *testing.T) {
	ctx := context.Background()

	// 1. Success case (0 orphan records)
	runner := &MockRunner{QueryCountResult: 0}
	a := &model.Asset{
		Name: "customer_360",
		Config: map[string]string{
			"test_relationship": "user_id to stg_users.user_id",
		},
	}

	execPlan := &ExecutionPlan{
		Environment: &state.Environment{Schema: "sqlforge__prod"},
		Unchanged: []*model.Asset{
			{Name: "stg_users", Config: map[string]string{"schema": "staging"}},
		},
	}

	err := RunDataQualityTests(ctx, runner, a, "sqlforge__prod", "customer_360", execPlan)
	if err != nil {
		t.Fatalf("Expected relationship test to pass, but failed: %v", err)
	}

	expectedSQL := "SELECT COUNT(*) FROM sqlforge__prod.customer_360 WHERE user_id IS NOT NULL AND user_id NOT IN (SELECT user_id FROM sqlforge__prod_staging.stg_users)"
	if len(runner.QueryCountCalls) != 1 || runner.QueryCountCalls[0] != expectedSQL {
		t.Errorf("Expected query:\n%s\nGot:\n%v", expectedSQL, runner.QueryCountCalls)
	}

	// 2. Failure case (1 orphan record)
	runnerFailure := &MockRunner{QueryCountResult: 1}
	err = RunDataQualityTests(ctx, runnerFailure, a, "sqlforge__prod", "customer_360", execPlan)
	if err == nil {
		t.Fatal("Expected relationship test to fail, but passed")
	}
	if !strings.Contains(err.Error(), "relationship validation failed") {
		t.Errorf("Expected error to mention relationship validation failed, got: %v", err)
	}
}

func TestRunSingularTests(t *testing.T) {
	ctx := context.Background()

	// 1. Success case (0 rows returned by assertion)
	runner := &MockRunner{QueryCountResult: 0}
	tests := []*model.Asset{
		{
			Name:         "assert_revenue_positive",
			Type:         "test",
			SQL:          "SELECT * FROM daily_metrics WHERE daily_revenue < 0",
			Dependencies: []string{"daily_metrics"},
		},
	}

	execPlan := &ExecutionPlan{
		Environment: &state.Environment{Schema: "sqlforge__prod"},
		Unchanged: []*model.Asset{
			{Name: "daily_metrics", Config: map[string]string{"schema": "marts"}},
		},
	}

	err := RunSingularTests(ctx, runner, tests, execPlan, nil)
	if err != nil {
		t.Fatalf("Expected singular test to pass, but failed: %v", err)
	}

	expectedSQL := "SELECT COUNT(*) FROM (\nSELECT * FROM sqlforge__prod_marts.daily_metrics WHERE daily_revenue < 0\n) AS _test_assertion"
	if len(runner.QueryCountCalls) != 1 || runner.QueryCountCalls[0] != expectedSQL {
		t.Errorf("Expected query:\n%s\nGot:\n%v", expectedSQL, runner.QueryCountCalls)
	}

	// 2. Failure case (rows returned by assertion)
	runnerFailure := &MockRunner{QueryCountResult: 5}
	err = RunSingularTests(ctx, runnerFailure, tests, execPlan, nil)
	if err == nil {
		t.Fatal("Expected singular test to fail, but passed")
	}
	if !strings.Contains(err.Error(), "singular assertion assert_revenue_positive returned 5 failing records") {
		t.Errorf("Expected error to mention singular assertion failures, got: %v", err)
	}
}
