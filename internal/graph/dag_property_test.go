package graph

import (
	"fmt"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"pgregory.net/rapid"
)

// Generate a random Directed Graph using rapid
func genGraph(t *rapid.T) []*model.Asset {
	numNodes := rapid.IntRange(1, 20).Draw(t, "numNodes")
	nodes := make([]*model.Asset, numNodes)

	for i := 0; i < numNodes; i++ {
		nodes[i] = &model.Asset{
			Name: fmt.Sprintf("node_%d", i),
		}
	}

	for i := 0; i < numNodes; i++ {
		numEdges := rapid.IntRange(0, numNodes-1).Draw(t, fmt.Sprintf("numEdges_%d", i))
		for j := 0; j < numEdges; j++ {
			depIdx := rapid.IntRange(0, numNodes-1).Draw(t, fmt.Sprintf("depIdx_%d_%d", i, j))
			nodes[i].Dependencies = append(nodes[i].Dependencies, fmt.Sprintf("node_%d", depIdx))
		}
	}

	return nodes
}

// Property: DetectCycle correctly identifies cycles
func TestProp_DAG_CycleDetection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		assets := genGraph(t)
		dag := NewDAG()
		err := dag.Build(assets)

		// Independent cycle detection using simple DFS
		hasCycle := false
		visited := make(map[string]bool)
		recStack := make(map[string]bool)

		var dfs func(node string) bool
		dfs = func(node string) bool {
			if recStack[node] {
				return true
			}
			if visited[node] {
				return false
			}

			visited[node] = true
			recStack[node] = true

			for _, a := range assets {
				if a.Name == node {
					for _, dep := range a.Dependencies {
						// Only check deps that are in the graph
						depExists := false
						for _, n := range assets {
							if n.Name == dep {
								depExists = true
								break
							}
						}
						if depExists && dfs(dep) {
							return true
						}
					}
					break
				}
			}

			recStack[node] = false
			return false
		}

		for _, a := range assets {
			if !visited[a.Name] {
				if dfs(a.Name) {
					hasCycle = true
					break
				}
			}
		}

		if hasCycle && err == nil {
			t.Fatalf("Expected cycle detection error, but got nil")
		} else if !hasCycle && err != nil {
			t.Fatalf("Expected no cycle error, but got: %v", err)
		}
	})
}

// Generate an acyclic DAG by only allowing edges from larger index to smaller index
func genDAG(t *rapid.T) []*model.Asset {
	numNodes := rapid.IntRange(1, 30).Draw(t, "numNodes")
	nodes := make([]*model.Asset, numNodes)

	for i := 0; i < numNodes; i++ {
		nodes[i] = &model.Asset{
			Name: fmt.Sprintf("node_%d", i),
		}
		if i > 0 {
			numEdges := rapid.IntRange(0, i).Draw(t, fmt.Sprintf("numEdges_%d", i))
			for j := 0; j < numEdges; j++ {
				depIdx := rapid.IntRange(0, i-1).Draw(t, fmt.Sprintf("depIdx_%d_%d", i, j))
				nodes[i].Dependencies = append(nodes[i].Dependencies, fmt.Sprintf("node_%d", depIdx))
			}
		}
	}

	return nodes
}

// Property: TopologicalSort respects order and completeness
func TestProp_DAG_TopologicalSort(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		assets := genDAG(t)
		dag := NewDAG()
		err := dag.Build(assets)
		if err != nil {
			t.Fatalf("Unexpected cycle in generated DAG: %v", err)
		}

		order, err := dag.TopologicalSort()
		if err != nil {
			t.Fatalf("Unexpected error in TopologicalSort: %v", err)
		}

		if len(order) != len(assets) {
			t.Fatalf("Expected %d nodes in topological sort, got %d", len(assets), len(order))
		}

		// Property 1: All nodes are present
		orderSet := make(map[string]bool)
		for _, node := range order {
			orderSet[node] = true
		}
		for _, a := range assets {
			if !orderSet[a.Name] {
				t.Fatalf("Node %s missing from topological sort", a.Name)
			}
		}

		// Property 2: Order is valid (dependencies come before dependents)
		position := make(map[string]int)
		for i, node := range order {
			position[node] = i
		}

		for _, a := range assets {
			for _, dep := range a.Dependencies {
				// Only check deps that are in the graph
				if _, exists := position[dep]; exists {
					if position[dep] > position[a.Name] {
						t.Fatalf("Dependency %s appears after %s in topological sort", dep, a.Name)
					}
				}
			}
		}
	})
}
