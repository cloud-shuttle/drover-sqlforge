package plan

import (
	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/state"
)

type ExecutionPlan struct {
	Environment   *state.Environment
	ChangedModels []*model.Asset
	Impacted      []*model.Asset
	Unchanged     []*model.Asset
}

func GeneratePlan(env *state.Environment, assets []*model.Asset, stateMgr *state.Manager, dag *graph.DAG) (*ExecutionPlan, error) {
	plan := &ExecutionPlan{
		Environment: env,
	}

	changedMap := make(map[string]bool)

	order, err := dag.TopologicalSort()
	if err != nil {
		return nil, err
	}

	for _, name := range order {
		a := dag.Nodes[name]
		fingerprint, err := graph.GenerateFingerprint(a.AST, a.Config)
		if err != nil {
			return nil, err
		}

		modelState, err := stateMgr.Store.GetModelState(a.Name, env.Name)
		if err != nil || modelState.Fingerprint != fingerprint {
			plan.ChangedModels = append(plan.ChangedModels, a)
			changedMap[a.Name] = true
		} else {
			plan.Unchanged = append(plan.Unchanged, a)
		}
	}

	// Detect impact
	visited := make(map[string]bool)
	var dfs func(node string)
	dfs = func(node string) {
		visited[node] = true
		// find nodes that depend on `node`
		for name, deps := range dag.Edges {
			for _, dep := range deps {
				if dep == node && !visited[name] {
					if !changedMap[name] {
						plan.Impacted = append(plan.Impacted, dag.Nodes[name])
						changedMap[name] = true // treat as changed for downstream
					}
					dfs(name)
				}
			}
		}
	}

	for _, a := range plan.ChangedModels {
		dfs(a.Name)
	}

	return plan, nil
}
