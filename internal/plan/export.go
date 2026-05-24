package plan

import (
	"fmt"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
)

type ExportNode struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Materialized string `json:"materialized"`
	Database     string `json:"database,omitempty"`
	Schema       string `json:"schema"`
	Command      string `json:"command"`
}

type ExportEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type ExportDAG struct {
	Environment string       `json:"environment"`
	Nodes       []ExportNode `json:"nodes"`
	Edges       []ExportEdge `json:"edges"`
}

// GenerateDAGExport serializes the runtime DAG into a generic ExportDAG format.
func GenerateDAGExport(envName string, envSchema string, assets []*model.Asset, dag *graph.DAG) *ExportDAG {
	export := &ExportDAG{
		Environment: envName,
		Nodes:       []ExportNode{},
		Edges:       []ExportEdge{},
	}

	// We use the full assets slice to ensure every node is captured.
	for _, a := range assets {
		mat := a.Config["materialized"]
		if mat == "" {
			mat = "view"
		}

		db, schema, _ := resolveTarget(envSchema, a)

		node := ExportNode{
			Name:         a.Name,
			Type:         a.Type,
			Materialized: mat,
			Database:     db,
			Schema:       schema,
			Command:      fmt.Sprintf("sqlforge apply %s --model %s", envName, a.Name),
		}
		export.Nodes = append(export.Nodes, node)

		// dag.Edges maps Node -> []Dependencies (i.e. to -> from).
		// Export edges should typically be Source -> Target.
		for _, dep := range a.Dependencies {
			export.Edges = append(export.Edges, ExportEdge{
				Source: dep,
				Target: a.Name,
			})
		}
	}

	return export
}
