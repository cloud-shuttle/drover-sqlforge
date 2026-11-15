package api

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"regexp"
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

// identifierRe matches a bare SQL identifier that is safe to interpolate
// into generated SQL without quoting.
var identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidIdentifier reports whether s is a bare SQL identifier (letters, digits
// and underscores, not starting with a digit, at most 128 characters).
func ValidIdentifier(s string) bool {
	return len(s) <= 128 && identifierRe.MatchString(s)
}

// previewModelName extracts {name} from /api/models/{name}/preview.
func previewModelName(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/models/")
	if !ok {
		return "", false
	}
	name, suffix, ok := strings.Cut(rest, "/")
	if !ok || suffix != "preview" || name == "" {
		return "", false
	}
	return name, true
}

// ServeModelPreview returns up to 50 rows of a materialised model. Only models
// present in the loaded DAG can be previewed, and both the schema and model
// name must be bare identifiers, so request input never reaches SQL text
// unchecked.
func ServeModelPreview(runner virtual.Runner, schema string, dag *graph.DAG) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		modelName, ok := previewModelName(r.URL.Path)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		if dag == nil {
			http.Error(w, "Model not found", http.StatusNotFound)
			return
		}
		if _, exists := dag.Nodes[modelName]; !exists {
			http.Error(w, "Model not found", http.StatusNotFound)
			return
		}
		if !ValidIdentifier(schema) || !ValidIdentifier(modelName) {
			http.Error(w, "Invalid model or schema name", http.StatusBadRequest)
			return
		}

		query := "SELECT * FROM " + schema + "." + modelName + " LIMIT 50"

		data, err := runner.QueryData(r.Context(), query)
		if err != nil {
			log.Printf("preview %s: query failed: %v", modelName, err)
			http.Error(w, "Query failed", http.StatusInternalServerError)
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

// IsLoopbackHost reports whether host (no port) is localhost or a loopback IP.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// LoopbackHostGuard rejects requests whose Host header is not a loopback
// name. It blocks DNS-rebinding attacks against a server bound to 127.0.0.1,
// where a malicious page resolves its own domain to the loopback address.
func LoopbackHostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !IsLoopbackHost(host) {
			http.Error(w, "Forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
