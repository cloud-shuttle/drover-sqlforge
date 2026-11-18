package plan

import (
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/graph"
	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
	"github.com/cloud-shuttle/drover-sqlforge/internal/parser"
	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
)

func asset(name, sql string) *model.Asset {
	return &model.Asset{
		Name:   name,
		SQL:    sql,
		AST:    &parser.ASTNode{Type: "select", Value: sql},
		Config: map[string]string{"materialized": "view"},
	}
}

func nameSet(assets []*model.Asset) map[string]bool {
	out := map[string]bool{}
	for _, a := range assets {
		out[a.Name] = true
	}
	return out
}

func TestGeneratePlanClassifiesChangedUnchangedAndImpacted(t *testing.T) {
	stg := asset("stg", "SELECT 1 AS id")
	mart := asset("mart", "SELECT id FROM stg")
	mart.Dependencies = []string{"stg"}
	dim := asset("dim", "SELECT 2 AS id")

	env := &state.Environment{Name: "prod", Schema: "sqlforge__prod"}
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []*model.Asset{mart, dim} {
		fp, err := graph.GenerateFingerprint(a.AST, a.Config)
		if err != nil {
			t.Fatal(err)
		}
		if err := stateMgr.Store.SaveModelState(&state.ModelState{
			ModelName:      a.Name,
			Environment:    env.Name,
			Fingerprint:    fp,
			MaterializedAs: "view",
		}); err != nil {
			t.Fatal(err)
		}
	}

	dag := graph.NewDAG()
	if err := dag.Build([]*model.Asset{stg, mart, dim}); err != nil {
		t.Fatal(err)
	}
	got, err := GeneratePlan(env, []*model.Asset{stg, mart, dim}, stateMgr, dag)
	if err != nil {
		t.Fatal(err)
	}
	if !nameSet(got.ChangedModels)["stg"] || len(got.ChangedModels) != 1 {
		t.Fatalf("changed: %v", nameSet(got.ChangedModels))
	}
	if !nameSet(got.Impacted)["mart"] || len(got.Impacted) != 1 {
		t.Fatalf("impacted: %v", nameSet(got.Impacted))
	}
	unchanged := nameSet(got.Unchanged)
	if !unchanged["dim"] || !unchanged["mart"] {
		t.Fatalf("unchanged: %v", unchanged)
	}
}

func TestGeneratePlanRejectsACycle(t *testing.T) {
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dag := graph.NewDAG()
	dag.AddAsset(asset("a", "SELECT 1"))
	dag.AddAsset(asset("b", "SELECT 1"))
	dag.AddEdge("a", "b")
	dag.AddEdge("b", "a")

	got, err := GeneratePlan(&state.Environment{Name: "prod"}, nil, stateMgr, dag)
	if err == nil || got != nil {
		t.Fatalf("plan=%v err=%v", got, err)
	}
}
