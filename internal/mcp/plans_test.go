package mcp

import (
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/plan"
	"github.com/drover-org/drover-sqlforge/internal/state"
)

func TestPlanStorePutGetDelete(t *testing.T) {
	store := NewPlanStore()
	p := &plan.ExecutionPlan{Environment: &state.Environment{Name: "dev"}}
	id, err := store.Put(p)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := store.Get(id)
	if !ok || got != p {
		t.Fatalf("plan not retrieved")
	}
	store.Delete(id)
	if _, ok := store.Get(id); ok {
		t.Fatal("plan should be deleted")
	}
}
