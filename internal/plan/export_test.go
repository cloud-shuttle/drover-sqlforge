package plan

import (
	"encoding/json"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
)

func TestGenerateDAGExport(t *testing.T) {
	assets := []*model.Asset{
		{
			Name: "stg_events",
			Type: "model",
			Config: map[string]string{
				"materialized": "view",
				"schema":       "marketing",
			},
			Dependencies: []string{},
		},
		{
			Name: "fct_events",
			Type: "model",
			Config: map[string]string{
				"materialized": "table",
			},
			Dependencies: []string{"stg_events"},
		},
	}

	dag := graph.NewDAG()
	if err := dag.Build(assets); err != nil {
		t.Fatal(err)
	}

	export := GenerateDAGExport("prod", "sqlforge__prod", assets, dag)

	if export.Environment != "prod" {
		t.Errorf("expected env prod, got %s", export.Environment)
	}

	if len(export.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(export.Nodes))
	}

	foundStg := false
	foundFct := false
	for _, n := range export.Nodes {
		if n.Name == "stg_events" {
			foundStg = true
			if n.Schema != "sqlforge__prod_marketing" {
				t.Errorf("expected schema sqlforge__prod_marketing, got %s", n.Schema)
			}
			if n.Materialized != "view" {
				t.Errorf("expected mat view, got %s", n.Materialized)
			}
			if n.Command != "sqlforge apply prod --model stg_events" {
				t.Errorf("expected command sqlforge apply prod --model stg_events, got %s", n.Command)
			}
		} else if n.Name == "fct_events" {
			foundFct = true
			if n.Schema != "sqlforge__prod" {
				t.Errorf("expected schema sqlforge__prod, got %s", n.Schema)
			}
			if n.Materialized != "table" {
				t.Errorf("expected mat table, got %s", n.Materialized)
			}
		}
	}

	if !foundStg || !foundFct {
		t.Errorf("missing expected nodes")
	}

	if len(export.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(export.Edges))
	}

	edge := export.Edges[0]
	if edge.Source != "stg_events" || edge.Target != "fct_events" {
		t.Errorf("expected edge from stg_events to fct_events, got %s -> %s", edge.Source, edge.Target)
	}

	// Verify JSON marshaling works cleanly
	_, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("failed to marshal export DAG: %v", err)
	}
}
