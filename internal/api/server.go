package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

type ReactFlowNode struct {
	ID       string                 `json:"id"`
	Position map[string]float64     `json:"position"`
	Data     map[string]interface{} `json:"data"`
}

type ReactFlowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type DAGResponse struct {
	Nodes []ReactFlowNode `json:"nodes"`
	Edges []ReactFlowEdge `json:"edges"`
}

func ServeDAG(dag *graph.DAG) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Enable CORS for local dev
		w.Header().Set("Access-Control-Allow-Origin", "*")

		resp := DAGResponse{
			Nodes: make([]ReactFlowNode, 0),
			Edges: make([]ReactFlowEdge, 0),
		}

		for _, node := range dag.Nodes {
			resp.Nodes = append(resp.Nodes, ReactFlowNode{
				ID: node.Name,
				Position: map[string]float64{
					"x": 0, // Frontend will use dagre for layout
					"y": 0,
				},
				Data: map[string]interface{}{
					"label": node.Name,
					"type":  node.Config["materialized"],
				},
			})
		}

		for from, tos := range dag.Edges {
			for _, to := range tos {
				resp.Edges = append(resp.Edges, ReactFlowEdge{
					ID:     "e-" + from + "-" + to,
					Source: from,
					Target: to,
				})
			}
		}

		json.NewEncoder(w).Encode(resp)
	}
}

func ServeModelDetails(dag *graph.DAG) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		modelName := parts[3] // /api/models/{name}

		asset, exists := dag.Nodes[modelName]
		if !exists {
			http.Error(w, "Model not found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(asset)
	}
}

func ServeModelPreview(runner virtual.Runner, schema string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 5 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		modelName := parts[3] // /api/models/{name}/preview

		// Execute query against the virtual environment
		query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 50", schema, modelName)
		
		data, err := runner.QueryData(context.Background(), query)
		if err != nil {
			http.Error(w, fmt.Sprintf("Query failed: %v", err), http.StatusInternalServerError)
			return
		}

		if data == nil {
			data = []map[string]interface{}{}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": modelName,
			"rows":  data,
		})
	}
}
