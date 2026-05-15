package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
)

func TestTools_ListModels(t *testing.T) {
	dag := &graph.DAG{
		Nodes: map[string]*model.Asset{
			"model_a": {
				Name: "model_a",
				Config: map[string]string{
					"materialized": "table",
				},
				AST: &parser.ASTNode{Type: "SelectStmt"},
			},
		},
	}

	reg := NewRegistry()
	reg.InitializeCoreTools(dag, nil)

	tool, ok := reg.Get("list_models")
	if !ok {
		t.Fatalf("tool list_models not found")
	}

	res, err := tool.Handler(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, ok := res.([]interface{})
	if ok {
		t.Logf("Warning: expected struct slice, got []interface{}: %v", list)
	}

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	
	var summaries []map[string]interface{}
	if err := json.Unmarshal(b, &summaries); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected 1 model summary, got %d", len(summaries))
	}
	if summaries[0]["name"] != "model_a" {
		t.Errorf("expected name model_a, got %v", summaries[0]["name"])
	}
}

func TestTools_GetModel(t *testing.T) {
	dag := &graph.DAG{
		Nodes: map[string]*model.Asset{
			"model_a": {
				Name: "model_a",
				Config: map[string]string{
					"materialized": "table",
				},
				SQL: "SELECT * FROM x",
			},
		},
	}

	reg := NewRegistry()
	reg.InitializeCoreTools(dag, nil)

	tool, _ := reg.Get("get_model")

	t.Run("Valid Model", func(t *testing.T) {
		res, err := tool.Handler(context.Background(), []byte(`{"model_name": "model_a"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		
		m := res.(map[string]interface{})
		if m["name"] != "model_a" {
			t.Errorf("expected name model_a, got %v", m["name"])
		}
		if m["sql"] != "SELECT * FROM x" {
			t.Errorf("expected SQL 'SELECT * FROM x', got %v", m["sql"])
		}
	})

	t.Run("Invalid Model", func(t *testing.T) {
		_, err := tool.Handler(context.Background(), []byte(`{"model_name": "missing"}`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Invalid Parameters", func(t *testing.T) {
		_, err := tool.Handler(context.Background(), []byte(`{invalid`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("No Context", func(t *testing.T) {
		reg2 := NewRegistry()
		reg2.InitializeCoreTools(nil, nil)
		tool2, _ := reg2.Get("get_model")
		_, err := tool2.Handler(context.Background(), []byte(`{"model_name": "model_a"}`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestTools_Metrics(t *testing.T) {
	semGraph := &semantic.Graph{
		Metrics: []semantic.Metric{
			{Name: "revenue"},
		},
	}

	reg := NewRegistry()
	reg.InitializeCoreTools(nil, semGraph)

	t.Run("List Metrics", func(t *testing.T) {
		tool, _ := reg.Get("list_metrics")
		res, err := tool.Handler(context.Background(), []byte(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		list := res.([]semantic.Metric)
		if len(list) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(list))
		}
	})

	t.Run("List Metrics No Context", func(t *testing.T) {
		reg2 := NewRegistry()
		reg2.InitializeCoreTools(nil, nil)
		tool2, _ := reg2.Get("list_metrics")
		_, err := tool2.Handler(context.Background(), []byte(`{}`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Query Metric - Not Found", func(t *testing.T) {
		tool, _ := reg.Get("query_metric")
		_, err := tool.Handler(context.Background(), []byte(`{"name":"unknown","dimensions":[]}`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Query Metric - Invalid Params", func(t *testing.T) {
		tool, _ := reg.Get("query_metric")
		_, err := tool.Handler(context.Background(), []byte(`{`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Query Metric - No Context", func(t *testing.T) {
		reg2 := NewRegistry()
		reg2.InitializeCoreTools(nil, nil)
		tool2, _ := reg2.Get("query_metric")
		_, err := tool2.Handler(context.Background(), []byte(`{"name":"revenue","dimensions":[]}`))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestTools_Placeholders(t *testing.T) {
	reg := NewRegistry()
	reg.InitializeCoreTools(nil, nil)

	tool, _ := reg.Get("plan_change")
	res, err := tool.Handler(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "Not Implemented" {
		t.Errorf("expected Not Implemented, got %v", res)
	}

	tool2, _ := reg.Get("apply_change")
	res2, err := tool2.Handler(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2 != "Not Implemented" {
		t.Errorf("expected Not Implemented, got %v", res2)
	}
}
