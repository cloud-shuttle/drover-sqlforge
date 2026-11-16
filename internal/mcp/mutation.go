package mcp

import (
	"context"
	"fmt"

	"github.com/cloud-shuttle/drover-sqlforge/internal/graph"
	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
	"github.com/cloud-shuttle/drover-sqlforge/internal/plan"
	"github.com/cloud-shuttle/drover-sqlforge/internal/project"
	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlguard"
)

func planChange(rt *project.Runtime, store *PlanStore, modelName, proposedSQL string) (map[string]interface{}, error) {
	if rt == nil || rt.DAG == nil {
		return nil, fmt.Errorf("no project runtime loaded")
	}

	existing, ok := rt.DAG.Nodes[modelName]
	if !ok {
		return nil, fmt.Errorf("model %s not found", modelName)
	}

	// Agent-proposed SQL is wrapped in CREATE ... AS and executed by
	// apply_change, so it must be a single read-only SELECT.
	if err := sqlguard.ValidateSelect(proposedSQL); err != nil {
		return nil, err
	}

	ast, err := rt.Parser.ParseToAST(proposedSQL)
	if err != nil {
		return nil, fmt.Errorf("parse proposed SQL: %w", err)
	}
	deps, err := rt.Parser.ExtractRefs(proposedSQL)
	if err != nil {
		return nil, fmt.Errorf("extract refs: %w", err)
	}

	modified := &model.Asset{
		Name:         existing.Name,
		Path:         existing.Path,
		Type:         existing.Type,
		Config:       copyConfig(existing.Config),
		SQL:          proposedSQL,
		AST:          ast,
		Dependencies: deps,
	}

	assets := replaceAsset(rt.Assets, modified)
	dag := graph.NewDAG()
	if err := dag.Build(assets); err != nil {
		return nil, fmt.Errorf("build dag: %w", err)
	}

	execPlan, err := plan.GeneratePlan(rt.Env, assets, rt.StateMgr, dag)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}

	planID, err := store.PutProposal(execPlan, modelName, proposedSQL)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plan_id":         planID,
		"environment":     rt.Env.Name,
		"model_name":      modelName,
		"changed_models":  assetNames(execPlan.ChangedModels),
		"impacted_models": assetNames(execPlan.Impacted),
		"unchanged_count": len(execPlan.Unchanged),
	}, nil
}

func applyChange(ctx context.Context, rt *project.Runtime, store *PlanStore, planID string) (map[string]interface{}, error) {
	if rt == nil {
		return nil, fmt.Errorf("no project runtime loaded")
	}

	pending, ok := store.GetPending(planID)
	if !ok {
		return nil, fmt.Errorf("plan_id %q not found or expired", planID)
	}
	execPlan := pending.Plan
	if err := revalidateProposal(pending); err != nil {
		store.Delete(planID)
		return nil, err
	}

	if len(execPlan.ChangedModels) == 0 && len(execPlan.Impacted) == 0 {
		return map[string]interface{}{
			"plan_id":     planID,
			"environment": rt.Env.Name,
			"status":      "noop",
			"message":     "nothing to apply",
		}, nil
	}

	if err := plan.ApplyPlan(ctx, execPlan, rt.StateMgr, rt.VMgr, rt.Parser, nil, 4); err != nil {
		return nil, err
	}
	store.Delete(planID)

	return map[string]interface{}{
		"plan_id":     planID,
		"environment": rt.Env.Name,
		"status":      "applied",
		"applied":     len(execPlan.ChangedModels) + len(execPlan.Impacted),
	}, nil
}

// revalidateProposal re-checks the stored proposal immediately before apply
// and confirms the plan executes exactly the SQL that was validated.
func revalidateProposal(p *PendingPlan) error {
	if p.ModelName == "" {
		return fmt.Errorf("plan has no recorded proposal; create it with plan_change")
	}
	if err := sqlguard.ValidateSelect(p.ProposedSQL); err != nil {
		return err
	}
	if p.Plan == nil {
		return nil
	}
	for _, group := range [][]*model.Asset{p.Plan.ChangedModels, p.Plan.Impacted} {
		for _, a := range group {
			if a != nil && a.Name == p.ModelName && a.SQL != p.ProposedSQL {
				return fmt.Errorf("plan for model %s does not match the validated proposal", p.ModelName)
			}
		}
	}
	return nil
}

func replaceAsset(assets []*model.Asset, modified *model.Asset) []*model.Asset {
	out := make([]*model.Asset, 0, len(assets))
	for _, a := range assets {
		if a.Name == modified.Name {
			out = append(out, modified)
		} else {
			out = append(out, a)
		}
	}
	return out
}

func copyConfig(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func assetNames(assets []*model.Asset) []string {
	names := make([]string, 0, len(assets))
	for _, a := range assets {
		names = append(names, a.Name)
	}
	return names
}
