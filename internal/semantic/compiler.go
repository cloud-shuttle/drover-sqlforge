package semantic

import (
	"fmt"
	"regexp"
	"strings"
)

type Compiler struct {
	EnvironmentSchema string // e.g. "sqlforge__peter_dev"
	// Relation, when set, is used verbatim as the FROM target instead of
	// EnvironmentSchema + "." + metric.Model. Callers set it to the model's
	// resolved warehouse name (folder/custom schema, alias, database).
	Relation string
}

func NewCompiler(schema string) *Compiler {
	return &Compiler{EnvironmentSchema: schema}
}

func (c *Compiler) Compile(metric *Metric, requestedDims []string) (string, error) {
	// Validate dimensions
	validDims := make(map[string]bool)
	dimRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

	for _, d := range metric.Dimensions {
		validDims[d] = true
	}

	for _, req := range requestedDims {
		if !dimRegex.MatchString(req) {
			return "", fmt.Errorf("dimension '%s' contains invalid characters", req)
		}
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

	from := metric.Model
	if c.Relation != "" {
		from = c.Relation
	} else if c.EnvironmentSchema != "" {
		from = c.EnvironmentSchema + "." + metric.Model
	}

	query := fmt.Sprintf("SELECT\n    %s\nFROM %s%s;", selectStr, from, groupStr)
	return query, nil
}
