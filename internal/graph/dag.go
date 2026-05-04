package graph

import (
	"fmt"

	"github.com/drover-org/drover-sqlforge/internal/model"
)

type DAG struct {
	Nodes map[string]*model.Asset
	Edges map[string][]string // from -> to (dependencies)
}

func NewDAG() *DAG {
	return &DAG{
		Nodes: make(map[string]*model.Asset),
		Edges: make(map[string][]string),
	}
}

func (d *DAG) AddAsset(a *model.Asset) {
	d.Nodes[a.Name] = a
}

func (d *DAG) AddEdge(from, to string) {
	d.Edges[from] = append(d.Edges[from], to)
}

func (d *DAG) Build(assets []*model.Asset) error {
	for _, a := range assets {
		d.AddAsset(a)
	}

	for _, a := range assets {
		for _, dep := range a.Dependencies {
			// In a full implementation, we'd resolve `dep` to an actual asset name
			// since it could be fully qualified. For now, assume it matches.
			if _, exists := d.Nodes[dep]; exists {
				d.AddEdge(a.Name, dep)
			}
		}
	}

	return d.DetectCycle()
}

func (d *DAG) DetectCycle() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string) error
	dfs = func(node string) error {
		if recStack[node] {
			return fmt.Errorf("cycle detected involving node: %s", node)
		}
		if visited[node] {
			return nil
		}

		visited[node] = true
		recStack[node] = true

		for _, neighbor := range d.Edges[node] {
			if err := dfs(neighbor); err != nil {
				return err
			}
		}

		recStack[node] = false
		return nil
	}

	for node := range d.Nodes {
		if !visited[node] {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *DAG) TopologicalSort() ([]string, error) {
	if err := d.DetectCycle(); err != nil {
		return nil, err
	}

	visited := make(map[string]bool)
	var stack []string

	var dfs func(node string)
	dfs = func(node string) {
		visited[node] = true
		for _, neighbor := range d.Edges[node] {
			if !visited[neighbor] {
				dfs(neighbor)
			}
		}
		stack = append(stack, node)
	}

	for node := range d.Nodes {
		if !visited[node] {
			dfs(node)
		}
	}

	return stack, nil
}
