package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

// qualityQuery pairs an assertion SQL (must return a single COUNT via
// runner.QueryCount) with a failure-message generator. onFail is only
// called when count > 0.
type qualityQuery struct {
	SQL    string
	onFail func(count int) error
}

// QualityCheck handles one category of data quality assertion declared in
// model config.
//
// Matches reports whether this check owns the given model config key.
// Queries generates the SQL assertions and failure generators for that
// key/value pair. Returning nil is valid (e.g. a malformed value).
type QualityCheck interface {
	Matches(key string) bool
	Queries(schema, table, key, val string, plan *ExecutionPlan) []qualityQuery
}

// registeredChecks is the ordered registry of all quality check types.
// Evaluation stops at the first matching check per config key.
var registeredChecks = []QualityCheck{
	notNullCheck{},
	uniqueCheck{},
	acceptedValuesCheck{},
	relationshipCheck{},
}

// RunDataQualityTestsFromConfig is the core implementation, called from
// ModelMaterializer.Apply. apply.go exports the backwards-compatible
// RunDataQualityTests shim that delegates here.
func RunDataQualityTestsFromConfig(ctx context.Context, runner virtual.Runner, cfg map[string]string, schema, table string, execPlan *ExecutionPlan) error {
	for k, v := range cfg {
		for _, check := range registeredChecks {
			if !check.Matches(k) {
				continue
			}
			for _, q := range check.Queries(schema, table, k, v, execPlan) {
				count, err := runner.QueryCount(ctx, q.SQL)
				if err != nil {
					return err
				}
				if count > 0 {
					return q.onFail(count)
				}
			}
			break // only one check type owns each key
		}
	}
	return nil
}

// ─── not_null ─────────────────────────────────────────────────────────────────

type notNullCheck struct{}

func (notNullCheck) Matches(key string) bool { return key == "test_not_null" }

func (notNullCheck) Queries(schema, table, _, val string, _ *ExecutionPlan) []qualityQuery {
	var out []qualityQuery
	for _, col := range splitCSV(val) {
		col := col // pin loop variable for closure
		out = append(out, qualityQuery{
			SQL: fmt.Sprintf(
				"SELECT COUNT(*) FROM %s.%s WHERE %s IS NULL",
				schema, table, col,
			),
			onFail: func(n int) error {
				return fmt.Errorf(
					"data quality test failed: %s.%s.%s is not_null but found %d null records",
					schema, table, col, n,
				)
			},
		})
	}
	return out
}

// ─── unique ───────────────────────────────────────────────────────────────────

type uniqueCheck struct{}

func (uniqueCheck) Matches(key string) bool { return key == "test_unique" }

func (uniqueCheck) Queries(schema, table, _, val string, _ *ExecutionPlan) []qualityQuery {
	var out []qualityQuery
	for _, col := range splitCSV(val) {
		col := col
		out = append(out, qualityQuery{
			SQL: fmt.Sprintf(
				"SELECT COUNT(*) FROM (SELECT %s, COUNT(*) as _c FROM %s.%s GROUP BY %s HAVING _c > 1)",
				col, schema, table, col,
			),
			onFail: func(n int) error {
				return fmt.Errorf(
					"data quality test failed: %s.%s.%s is unique but found %d duplicate records",
					schema, table, col, n,
				)
			},
		})
	}
	return out
}

// ─── accepted_values ──────────────────────────────────────────────────────────

type acceptedValuesCheck struct{}

func (acceptedValuesCheck) Matches(key string) bool {
	return strings.HasPrefix(key, "test_accepted_values_")
}

func (acceptedValuesCheck) Queries(schema, table, key, val string, _ *ExecutionPlan) []qualityQuery {
	col := strings.TrimPrefix(key, "test_accepted_values_")
	var quoted []string
	for _, v := range splitCSV(val) {
		quoted = append(quoted, fmt.Sprintf("'%s'", v))
	}
	inClause := strings.Join(quoted, ", ")
	return []qualityQuery{{
		SQL: fmt.Sprintf(
			"SELECT COUNT(*) FROM %s.%s WHERE %s NOT IN (%s)",
			schema, table, col, inClause,
		),
		onFail: func(n int) error {
			return fmt.Errorf(
				"data quality test failed: %s.%s.%s contains %d records not in accepted values (%s)",
				schema, table, col, n, inClause,
			)
		},
	}}
}

// ─── relationship ─────────────────────────────────────────────────────────────

type relationshipCheck struct{}

func (relationshipCheck) Matches(key string) bool {
	return strings.HasPrefix(key, "test_relationship")
}

func (relationshipCheck) Queries(schema, table, _, val string, execPlan *ExecutionPlan) []qualityQuery {
	// Parse "localCol to parentModel.parentCol" or "localCol->parentModel.parentCol"
	parts := strings.Split(val, " to ")
	if len(parts) != 2 {
		parts = strings.Split(val, "->")
	}
	if len(parts) != 2 {
		return nil // malformed value — skip silently (matches prior behaviour)
	}

	localCol := strings.TrimSpace(parts[0])
	parentParts := strings.Split(strings.TrimSpace(parts[1]), ".")
	if len(parentParts) != 2 {
		return nil
	}
	parentModel := strings.TrimSpace(parentParts[0])
	parentCol := strings.TrimSpace(parentParts[1])

	envSchema := schema
	if execPlan != nil {
		envSchema = execPlan.Environment.Schema
	}
	parentTableFQN := ResolveParentTarget(envSchema, parentModel, execPlan)

	return []qualityQuery{{
		SQL: fmt.Sprintf(
			"SELECT COUNT(*) FROM %s.%s WHERE %s IS NOT NULL AND %s NOT IN (SELECT %s FROM %s)",
			schema, table, localCol, localCol, parentCol, parentTableFQN,
		),
		onFail: func(n int) error {
			return fmt.Errorf(
				"data quality test failed: relationship validation failed between %s.%s.%s and %s.%s (found %d invalid records)",
				schema, table, localCol, parentTableFQN, parentCol, n,
			)
		},
	}}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// splitCSV splits a comma-separated string and trims whitespace from each
// element. Empty elements are dropped.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
