package plan

import (
	"encoding/json"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
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

func TestGenerateCatalogExport(t *testing.T) {
	assets := []*model.Asset{
		{
			Name: "stg_orders",
			Type: "model",
			Path: "models/staging/stg_orders.sql",
			Config: map[string]string{
				"materialized":  "view",
				"test_not_null": "id, customer_id",
				"test_unique":   "id",
			},
			SQL:          "SELECT id, customer_id FROM raw_orders",
			Dependencies: []string{},
		},
		{
			Name: "fct_orders",
			Type: "model",
			Path: "models/marts/fct_orders.sql",
			Config: map[string]string{
				"materialized":      "table",
				"test_relationship": "customer_id to stg_orders.customer_id",
			},
			SQL:          "SELECT id, customer_id FROM stg_orders",
			Dependencies: []string{"stg_orders"},
		},
	}

	dag := graph.NewDAG()
	if err := dag.Build(assets); err != nil {
		t.Fatal(err)
	}

	semGraph := &semantic.Graph{
		Metrics: []semantic.Metric{
			{
				Name:       "order_count",
				Expression: "COUNT(id)",
				Model:      "fct_orders",
				Dimensions: []string{"customer_id"},
			},
		},
	}

	// We pass nil for parser.Parser for unit testing without full WASM load
	export := GenerateCatalogExport("prod", "sqlforge__prod", assets, dag, semGraph, nil)

	if export.Environment != "prod" {
		t.Errorf("expected env prod, got %s", export.Environment)
	}

	if export.Schema != "sqlforge__prod" {
		t.Errorf("expected schema sqlforge__prod, got %s", export.Schema)
	}

	if len(export.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(export.Models))
	}

	stg, ok := export.Models["stg_orders"]
	if !ok {
		t.Fatalf("missing stg_orders")
	}
	if stg.Materialization != "view" {
		t.Errorf("expected view, got %s", stg.Materialization)
	}
	if len(stg.Tests) != 3 { // not_null(id), not_null(customer_id), unique(id)
		t.Errorf("expected 3 tests for stg_orders, got %d: %v", len(stg.Tests), stg.Tests)
	}

	fct, ok := export.Models["fct_orders"]
	if !ok {
		t.Fatalf("missing fct_orders")
	}
	if fct.Materialization != "table" {
		t.Errorf("expected table, got %s", fct.Materialization)
	}
	if len(fct.Tests) != 1 || fct.Tests[0] != "relationship(customer_id -> stg_orders.customer_id)" {
		t.Errorf("expected relationship test for fct_orders, got: %v", fct.Tests)
	}

	if len(export.Metrics) != 1 || export.Metrics[0].Name != "order_count" {
		t.Errorf("expected 1 metric order_count, got: %v", export.Metrics)
	}

	// Verify JSON marshaling works cleanly
	_, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("failed to marshal catalog export: %v", err)
	}
}
