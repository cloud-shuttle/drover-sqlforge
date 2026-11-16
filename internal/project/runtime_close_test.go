package project

import "testing"

type closingMockRunner struct {
	mockRunner
	closed int
}

func (c *closingMockRunner) Close() error { c.closed++; return nil }

// TestRuntimeClose_StopsRunner guards the plugin-leak fix: closing the runtime
// must close a runner that holds resources (a plugin subprocess).
func TestRuntimeClose_StopsRunner(t *testing.T) {
	stateMgr, env := newTestEnv(t)
	runner := &closingMockRunner{mockRunner: mockRunner{name: "duckdb"}}
	rt, err := NewRuntime(".", RuntimeDeps{StateMgr: stateMgr, Runner: runner, Env: env})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	rt.Close()
	if runner.closed != 1 {
		t.Fatalf("runner closed %d times, want 1", runner.closed)
	}

	// A runtime without a runner (VMgr nil) closes cleanly.
	bare, err := NewRuntime(".", RuntimeDeps{StateMgr: stateMgr, Env: env})
	if err != nil {
		t.Fatal(err)
	}
	bare.Close()
}
