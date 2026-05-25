package plan

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
)

//go:embed docs_template.html
var DocsTemplate string

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

		db, schema, _ := ResolveTarget(envSchema, a)

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

// CatalogModel represents a parsed model inside the static data catalog.
type CatalogModel struct {
	Name            string                 `json:"name"`
	Path            string                 `json:"path"`
	Type            string                 `json:"type"` // e.g. "model", "source", "metric"
	Materialization string                 `json:"materialization"`
	Database        string                 `json:"database,omitempty"`
	Schema          string                 `json:"schema"`
	SQL             string                 `json:"sql"`
	Lineage         []parser.ColumnMapping `json:"lineage"`
	Upstream        []string               `json:"upstream"`
	Downstream      []string               `json:"downstream"`
	Tests           []string               `json:"tests"`
	Config          map[string]string      `json:"config"`
}

// CatalogMetric represents a semantic metric inside the static data catalog.
type CatalogMetric struct {
	Name       string   `json:"name"`
	Expression string   `json:"expression"`
	Model      string   `json:"model"`
	Dimensions []string `json:"dimensions"`
}

// CatalogExport holds the entire structural catalog.
type CatalogExport struct {
	Environment string                   `json:"environment"`
	Schema      string                   `json:"schema"`
	Models      map[string]*CatalogModel `json:"models"`
	Metrics     []CatalogMetric          `json:"metrics"`
	Edges       []ExportEdge             `json:"edges"`
}

// GenerateCatalogExport compiles the static data catalog metadata from the runtime state.
func GenerateCatalogExport(
	envName string,
	envSchema string,
	assets []*model.Asset,
	dag *graph.DAG,
	semGraph *semantic.Graph,
	p *parser.Parser,
) *CatalogExport {
	export := &CatalogExport{
		Environment: envName,
		Schema:      envSchema,
		Models:      make(map[string]*CatalogModel),
		Metrics:     []CatalogMetric{},
		Edges:       []ExportEdge{},
	}

	// Populate Downstream relations.
	downstreams := make(map[string][]string)
	for _, a := range assets {
		for _, dep := range a.Dependencies {
			downstreams[dep] = append(downstreams[dep], a.Name)
		}
	}

	for _, a := range assets {
		mat := a.Config["materialized"]
		if mat == "" {
			mat = "view"
		}

		db, schema, _ := ResolveTarget(envSchema, a)

		// Parse column-level lineage
		var lineage []parser.ColumnMapping
		if p != nil && a.SQL != "" {
			if l, err := p.ExtractColumnLineage(a.SQL); err == nil {
				lineage = l
			}
		}

		// Parse declared data quality assertions
		var tests []string
		for k, v := range a.Config {
			if k == "test_not_null" {
				cols := strings.Split(v, ",")
				for _, col := range cols {
					tests = append(tests, fmt.Sprintf("not_null(%s)", strings.TrimSpace(col)))
				}
			} else if k == "test_unique" {
				cols := strings.Split(v, ",")
				for _, col := range cols {
					tests = append(tests, fmt.Sprintf("unique(%s)", strings.TrimSpace(col)))
				}
			} else if strings.HasPrefix(k, "test_accepted_values_") {
				col := strings.TrimPrefix(k, "test_accepted_values_")
				tests = append(tests, fmt.Sprintf("accepted_values(%s: %s)", col, v))
			} else if strings.HasPrefix(k, "test_relationship") {
				parts := strings.Split(v, " to ")
				if len(parts) != 2 {
					parts = strings.Split(v, "->")
				}
				if len(parts) == 2 {
					localCol := strings.TrimSpace(parts[0])
					parentTarget := strings.TrimSpace(parts[1])
					tests = append(tests, fmt.Sprintf("relationship(%s -> %s)", localCol, parentTarget))
				} else {
					tests = append(tests, fmt.Sprintf("relationship(%s)", v))
				}
			}
		}

		// Build CatalogModel representation
		modelExport := &CatalogModel{
			Name:            a.Name,
			Path:            a.Path,
			Type:            a.Type,
			Materialization: mat,
			Database:        db,
			Schema:          schema,
			SQL:             a.SQL,
			Lineage:         lineage,
			Upstream:        a.Dependencies,
			Downstream:      downstreams[a.Name],
			Tests:           tests,
			Config:          a.Config,
		}

		export.Models[a.Name] = modelExport

		for _, dep := range a.Dependencies {
			export.Edges = append(export.Edges, ExportEdge{
				Source: dep,
				Target: a.Name,
			})
		}
	}

	if semGraph != nil {
		for _, m := range semGraph.Metrics {
			export.Metrics = append(export.Metrics, CatalogMetric{
				Name:       m.Name,
				Expression: m.Expression,
				Model:      m.Model,
				Dimensions: m.Dimensions,
			})
		}
	}

	return export
}

