package api

import (
	"encoding/json"
	"net/http"

	"github.com/drover-org/drover-sqlforge/internal/graph"
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
