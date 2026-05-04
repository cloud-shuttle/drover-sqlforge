package state

import (
	"os"
	"testing"
	"time"
)

func TestStateStore(t *testing.T) {
	tmpDir := t.TempDir()
	
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.db.Close()
	defer os.RemoveAll(tmpDir)

	env := &Environment{
		Name:      "test_env",
		Schema:    "test_schema",
		CreatedAt: time.Now(),
		IsVirtual: true,
		BaseEnv:   "prod",
	}

	if err := store.SaveEnvironment(env); err != nil {
		t.Fatalf("Failed to save env: %v", err)
	}

	fetchedEnv, err := store.GetEnvironment("test_env")
	if err != nil {
		t.Fatalf("Failed to get env: %v", err)
	}

	if fetchedEnv.Schema != "test_schema" {
		t.Errorf("Expected schema test_schema, got %s", fetchedEnv.Schema)
	}

	modelState := &ModelState{
		ModelName:      "test_model",
		Fingerprint:    "hash123",
		LastApplied:    time.Now(),
		MaterializedAs: "table",
		Environment:    "test_env",
	}

	if err := store.SaveModelState(modelState); err != nil {
		t.Fatalf("Failed to save model state: %v", err)
	}

	fetchedState, err := store.GetModelState("test_model", "test_env")
	if err != nil {
		t.Fatalf("Failed to get model state: %v", err)
	}

	if fetchedState.Fingerprint != "hash123" {
		t.Errorf("Expected fingerprint hash123, got %s", fetchedState.Fingerprint)
	}
}
