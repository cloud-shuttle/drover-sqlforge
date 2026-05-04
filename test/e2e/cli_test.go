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
}
