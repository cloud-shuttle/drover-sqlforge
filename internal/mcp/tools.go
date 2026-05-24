package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/project"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
)

// ToolSchema defines the JSON schema for the tool parameters
type ToolSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]SchemaProperty `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type SchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Tool represents an MCP Tool exposed to the agent
type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema ToolSchema `json:"inputSchema"`

	// Handler is the function executed when the tool is called
	Handler func(ctx context.Context, params []byte) (interface{}, error) `json:"-"`
}

// Registry holds all registered MCP tools
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) ListTools() []Tool {
	var list []Tool
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// InitializeCoreTools registers the core SQLForge tools.
func (r *Registry) InitializeCoreTools(rt *project.Runtime, plans *PlanStore) {
	var dag *graph.DAG
	var semGraph *semantic.Graph
	if rt != nil {
		dag = rt.DAG
		semGraph = rt.Semantic
	}
	r.Register(Tool{
		Name:        "list_models",
		Description: "Returns all models in the project with metadata and fingerprints",
		InputSchema: ToolSchema{
			Type:       "object",
			Properties: map[string]SchemaProperty{},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			type ModelSummary struct {
				Name         string            `json:"name"`
				Materialized string            `json:"materialized"`
				Fingerprint  string            `json:"fingerprint"`
				Config       map[string]string `json:"config"`
			}
			var summaries []ModelSummary
			if dag != nil {
				for _, node := range dag.Nodes {
					fp, _ := graph.GenerateFingerprint(node.AST, node.Config)
					summaries = append(summaries, ModelSummary{
						Name:         node.Name,
						Materialized: node.Config["materialized"],
						Fingerprint:  fp,
						Config:       node.Config,
					})
				}
			}
			return summaries, nil
		},
	})

	r.Register(Tool{
		Name:        "get_model",
		Description: "Returns full model details including AST summary and lineage",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"model_name": {Type: "string", Description: "The name of the model to retrieve"},
			},
			Required: []string{"model_name"},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			var args struct {
				ModelName string `json:"model_name"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("invalid parameters")
			}

			if dag == nil {
				return nil, fmt.Errorf("no project context available")
			}

			node, exists := dag.Nodes[args.ModelName]
			if !exists {
				return nil, fmt.Errorf("model %s not found", args.ModelName)
			}

			fp, _ := graph.GenerateFingerprint(node.AST, node.Config)

			// Build response
			resp := map[string]interface{}{
				"name":         node.Name,
				"path":         node.Path,
				"type":         node.Type,
				"config":       node.Config,
				"sql":          node.SQL,
				"dependencies": node.Dependencies,
				"fingerprint":  fp,
			}

			// Add AST summary if available
			if node.AST != nil {
				resp["ast_type"] = node.AST.Type
				resp["ast_value"] = node.AST.Value
			}

			if rt != nil && rt.Parser != nil {
				if cols, err := rt.Parser.ExtractColumnLineage(node.SQL); err == nil {
					resp["column_lineage"] = cols
				}
			}

			return resp, nil
		},
	})

	r.Register(Tool{
		Name:        "list_metrics",
		Description: "List all semantic layer metrics available for querying",
		InputSchema: ToolSchema{
			Type:       "object",
			Properties: map[string]SchemaProperty{},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			if semGraph == nil {
				return nil, fmt.Errorf("semantic layer not loaded")
			}
			return semGraph.Metrics, nil
		},
	})

	r.Register(Tool{
		Name:        "query_metric",
		Description: "Compile a semantic metric into raw SQL based on dimensions",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"name":       {Type: "string", Description: "The metric name"},
				"dimensions": {Type: "array", Description: "List of dimensions to group by"},
			},
			Required: []string{"name"},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			var args struct {
				Name       string   `json:"name"`
				Dimensions []string `json:"dimensions"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("invalid parameters")
			}

			if semGraph == nil {
				return nil, fmt.Errorf("semantic layer not loaded")
			}

			metric := semGraph.FindMetric(args.Name)
			if metric == nil {
				return nil, fmt.Errorf("metric %s not found", args.Name)
			}

			compiler := semantic.NewCompiler("")
			sql, err := compiler.Compile(metric, args.Dimensions)
			if err != nil {
				return nil, fmt.Errorf("failed to compile metric: %v", err)
			}

			return map[string]interface{}{
				"metric":       metric,
				"dimensions":   args.Dimensions,
				"compiled_sql": sql,
			}, nil
		},
	})

	r.Register(Tool{
		Name:        "plan_change",
		Description: "Propose new model SQL and return an execution plan with plan_id for apply_change",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"model_name":   {Type: "string", Description: "Model to change"},
				"proposed_sql": {Type: "string", Description: "The new SQL content"},
			},
			Required: []string{"model_name", "proposed_sql"},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			var args struct {
				ModelName   string `json:"model_name"`
				ProposedSQL string `json:"proposed_sql"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("invalid parameters")
			}
			if args.ModelName == "" || args.ProposedSQL == "" {
				return nil, fmt.Errorf("model_name and proposed_sql are required")
			}
			return planChange(rt, plans, args.ModelName, args.ProposedSQL)
		},
	})

	r.Register(Tool{
		Name:        "apply_change",
		Description: "Execute a plan_id returned by plan_change",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"plan_id": {Type: "string", Description: "plan_id from plan_change"},
			},
			Required: []string{"plan_id"},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			var args struct {
				PlanID string `json:"plan_id"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("invalid parameters")
			}
			if args.PlanID == "" {
				return nil, fmt.Errorf("plan_id is required")
			}
			return applyChange(ctx, rt, plans, args.PlanID)
		},
	})

	r.registerSnapshotTools(rt)
}
