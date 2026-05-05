package graph

import (
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
)

func TestDAGCycleDetection(t *testing.T) {
	dag := NewDAG()

	a1 := &model.Asset{Name: "a1", Dependencies: []string{"a2"}}
	a2 := &model.Asset{Name: "a2", Dependencies: []string{"a1"}}

	err := dag.Build([]*model.Asset{a1, a2})
	if err == nil {
		t.Errorf("Expected cycle detection error, got nil")
	}
}

func TestDAGTopologicalSort(t *testing.T) {
	dag := NewDAG()

	a1 := &model.Asset{Name: "a1", Dependencies: []string{}}
	a2 := &model.Asset{Name: "a2", Dependencies: []string{"a1"}}

	err := dag.Build([]*model.Asset{a1, a2})
	if err != nil {
		t.Fatalf("Unexpected error building DAG: %v", err)
	}

	order, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("Unexpected error during topological sort: %v", err)
	}

	if len(order) != 2 || order[0] != "a1" || order[1] != "a2" {
		t.Errorf("Unexpected topological order: %v", order)
	}
}
