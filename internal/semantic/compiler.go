package semantic

import (
	"fmt"
	"strings"
)

type Compiler struct {
	EnvironmentSchema string // e.g. "sqlforge__peter_dev"
}

func NewCompiler(schema string) *Compiler {
	return &Compiler{EnvironmentSchema: schema}
}

func (c *Compiler) Compile(metric *Metric, requestedDims []string) (string, error) {
	// Validate dimensions
	validDims := make(map[string]bool)
	for _, d := range metric.Dimensions {
		validDims[d] = true
	}

	for _, req := range requestedDims {
		if !validDims[req] {
			return "", fmt.Errorf("dimension '%s' is not supported by metric '%s'", req, metric.Name)
		}
	}

	var selects []string
	var groups []string

	// Add requested dimensions
	for _, dim := range requestedDims {
		selects = append(selects, dim)
		groups = append(groups, dim)
	}

	// Add the metric expression itself
	selects = append(selects, fmt.Sprintf("%s AS %s", metric.Expression, metric.Name))

	selectStr := strings.Join(selects, ",\n    ")
	
	groupStr := ""
	if len(groups) > 0 {
		groupStr = "\nGROUP BY " + strings.Join(groups, ", ")
	}

	query := fmt.Sprintf("SELECT\n    %s\nFROM %s.%s%s;", selectStr, c.EnvironmentSchema, metric.Model, groupStr)
	return query, nil
}
