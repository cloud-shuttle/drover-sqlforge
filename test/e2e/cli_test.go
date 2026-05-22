package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/state"
)

func TestE2EPipeline(t *testing.T) {
	// Find absolute path to project root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}

	// Assuming we are in test/e2e
	projectRoot := filepath.Join(cwd, "..", "..")
	cliPath := filepath.Join(projectRoot, "sqlforge")

	// Ensure the binary exists
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Fatalf("sqlforge binary not found at %s. Please run 'make e2e' to build the binary before testing.", cliPath)
	}

	exampleDir := filepath.Join(projectRoot, "examples", "agentic_retail_2026")

	// 1. Clean previous state
	stateDir := filepath.Join(exampleDir, ".sqlforge")
	os.RemoveAll(stateDir)
	defer os.RemoveAll(stateDir) // cleanup after test

	// Helper to run CLI
	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(cliPath, args...)
		cmd.Dir = exampleDir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), err
	}

	// 2. Run 'plan prod'
	out, err := runCLI("plan", "prod")
	if err != nil {
		t.Fatalf("Failed to run plan prod: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(out, "Generating plan for environment: prod") {
		t.Errorf("Expected plan output to indicate prod environment, got: %s", out)
	}
	if !strings.Contains(out, "Changed Models:") {
		t.Errorf("Expected plan output to list changed models, got: %s", out)
	}

	// 3. Run 'apply prod'
	out, err = runCLI("apply", "prod")
	if err != nil {
		t.Fatalf("Failed to run apply prod: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(out, "Apply completed successfully.") {
		t.Errorf("Expected apply output to indicate success, got: %s", out)
	}

	// 4. Verify state was created
	stateMgr, err := state.NewManager(exampleDir)
	if err != nil {
		t.Fatalf("Failed to create state manager to verify state: %v", err)
	}
	// Note: manager uses an embedded db, we don't have Close on Manager but store.db is kept open.
	// Since tests exit, we don't strictly need to close, but let's query.

	// Check if environment prod was created
	env, err := stateMgr.GetOrCreateEnv("prod", "prod") // this will get if exists
	if err != nil {
		t.Fatalf("Expected prod environment to exist: %v", err)
	}
	if env.Schema != "sqlforge__prod" {
		t.Errorf("Expected schema sqlforge__prod, got %s", env.Schema)
	}

	// Check if customer_360 model state was saved
	st, err := stateMgr.Store.GetModelState("customer_360", "prod")
	if err != nil {
		t.Fatalf("Failed to retrieve state for customer_360: %v", err)
	}

	if st.MaterializedAs != "table" {
		t.Errorf("Expected customer_360 to be materialized as table, got %s", st.MaterializedAs)
	}

	// 5. Test Semantic Query CLI
	out, err = runCLI("query", "daily_active_users", "prod", "--dimensions", "metric_date")
	if err != nil {
		t.Fatalf("Failed to run semantic query: %v\nOutput: %s", err, out)
	}
	if !strings.Contains(out, "GROUP BY metric_date") {
		t.Errorf("Expected semantic query to output GROUP BY metric_date, got: %s", out)
	}

	// 6. Test Data Quality Failure
	// Create a intentionally failing model
	badModelPath := filepath.Join(exampleDir, "models", "staging", "stg_bad.sql")
	badModelSQL := `-- @materialized: table
-- @test_not_null: user_id
SELECT NULL AS user_id;`
	if err := os.WriteFile(badModelPath, []byte(badModelSQL), 0644); err != nil {
		t.Fatalf("Failed to write bad model: %v", err)
	}
	defer os.Remove(badModelPath)

	// Run plan & apply to trigger the test failure
	_, _ = runCLI("plan", "prod")
	out, err = runCLI("apply", "prod")
	if err == nil {
		t.Fatalf("Expected apply to fail due to data quality test on stg_bad, but it succeeded.\nOutput: %s", out)
	}
	if !strings.Contains(out, "data quality test failed") {
		t.Errorf("Expected error to mention data quality test failed, got: %s", out)
	}
}

func TestE2ESnapshot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	cliPath := filepath.Join(projectRoot, "sqlforge")
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Fatalf("sqlforge binary not found at %s. Run 'make e2e' first.", cliPath)
	}

	exampleDir := filepath.Join(projectRoot, "examples", "agentic_retail_2026")
	stateDir := filepath.Join(exampleDir, ".sqlforge")
	os.RemoveAll(stateDir)
	defer os.RemoveAll(stateDir)

	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(cliPath, args...)
		cmd.Dir = exampleDir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), err
	}

	out, err := runCLI("snapshot", "prod")
	if err != nil {
		t.Fatalf("snapshot prod failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "users_snapshot: ok") {
		t.Errorf("expected users_snapshot success, got: %s", out)
	}
	if !strings.Contains(out, "initial build") {
		t.Errorf("expected initial build on first run, got: %s", out)
	}

	stateMgr, err := state.NewManager(exampleDir)
	if err != nil {
		t.Fatalf("state manager: %v", err)
	}
	st, err := stateMgr.Store.GetSnapshotState("users_snapshot", "prod")
	if err != nil {
		t.Fatalf("expected snapshot state: %v", err)
	}
	if st.Strategy != "timestamp" || st.Fingerprint == "" {
		t.Errorf("unexpected snapshot state: %+v", st)
	}

	out, err = runCLI("snapshot", "prod", "users_snapshot")
	if err != nil {
		t.Fatalf("second snapshot run failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "incremental run") {
		t.Errorf("expected incremental run on second apply, got: %s", out)
	}
}

func TestE2EEnvCreate(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	cliPath := filepath.Join(projectRoot, "sqlforge")
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Fatalf("sqlforge binary not found at %s. Run 'make e2e' first.", cliPath)
	}

	exampleDir := filepath.Join(projectRoot, "examples", "agentic_retail_2026")
	stateDir := filepath.Join(exampleDir, ".sqlforge")
	os.RemoveAll(stateDir)
	defer os.RemoveAll(stateDir)

	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(cliPath, args...)
		cmd.Dir = exampleDir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), err
	}

	out, err := runCLI("env", "create", "preview_e2e", "--base-env", "prod")
	if err != nil {
		t.Fatalf("env create failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Created environment preview_e2e") {
		t.Errorf("expected create confirmation, got: %s", out)
	}
	if !strings.Contains(out, "sqlforge__preview_e2e") {
		t.Errorf("expected schema in output, got: %s", out)
	}

	stateMgr, err := state.NewManager(exampleDir)
	if err != nil {
		t.Fatalf("state manager: %v", err)
	}
	env, err := stateMgr.Store.GetEnvironment("preview_e2e")
	if err != nil {
		t.Fatalf("expected preview_e2e in state: %v", err)
	}
	if env.Schema != "sqlforge__preview_e2e" || env.BaseEnv != "prod" {
		t.Errorf("unexpected env: %+v", env)
	}

	out, err = runCLI("env", "create", "preview_e2e", "--base-env", "prod")
	if err != nil {
		t.Fatalf("idempotent env create failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Created environment preview_e2e") {
		t.Errorf("expected idempotent create message, got: %s", out)
	}
}

func TestE2ELineage(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	cliPath := filepath.Join(projectRoot, "sqlforge")
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Fatalf("sqlforge binary not found at %s. Run 'make e2e' first.", cliPath)
	}

	exampleDir := filepath.Join(projectRoot, "examples", "agentic_retail_2026")
	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(cliPath, args...)
		cmd.Dir = exampleDir
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), err
	}

	out, err := runCLI("lineage", "customer_360")
	if err != nil {
		t.Fatalf("lineage failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "customer_360") {
		t.Errorf("expected model header, got: %s", out)
	}
	if !strings.Contains(out, "user_id <- stg_users.user_id") {
		t.Errorf("expected column lineage edge, got: %s", out)
	}
}
