package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlident"
	"github.com/cloud-shuttle/drover-sqlforge/internal/virtual"
)

// qualityQuery pairs an assertion SQL (must return a single COUNT via
// runner.QueryCount) with a failure-message generator. onFail is only
// called when count > 0.
type qualityQuery struct {
	SQL    string
	onFail func(count int) error
	// err is set instead of SQL when the config value is unsafe to
	// interpolate (e.g. an invalid column name).
	err error
}

// QualityCheck handles one category of data quality assertion declared in
// model config.
//
// Matches reports whether this check owns the given model config key.
// Queries generates the SQL assertions and failure generators for that
// key/value pair. Returning nil is valid (e.g. a malformed value). dialect is
// the runner name, used to escape string literals.
type QualityCheck interface {
	Matches(key string) bool
	Queries(dialect, schema, table, key, val string, plan *ExecutionPlan) []qualityQuery
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
			for _, q := range check.Queries(runner.Name(), schema, table, k, v, execPlan) {
				if q.err != nil {
					return fmt.Errorf("data quality test %s: %w", k, q.err)
				}
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

func (notNullCheck) Queries(_, schema, table, _, val string, _ *ExecutionPlan) []qualityQuery {
	var out []qualityQuery
	for _, col := range splitCSV(val) {
		col := col // pin loop variable for closure
		if err := sqlident.ValidateIdentifier("not_null column", col); err != nil {
			return []qualityQuery{{err: err}}
		}
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

func (uniqueCheck) Queries(_, schema, table, _, val string, _ *ExecutionPlan) []qualityQuery {
	var out []qualityQuery
	for _, col := range splitCSV(val) {
		col := col
		if err := sqlident.ValidateIdentifier("unique column", col); err != nil {
			return []qualityQuery{{err: err}}
		}
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

func (acceptedValuesCheck) Queries(dialect, schema, table, key, val string, _ *ExecutionPlan) []qualityQuery {
	col := strings.TrimPrefix(key, "test_accepted_values_")
	if err := sqlident.ValidateIdentifier("accepted_values column", col); err != nil {
		return []qualityQuery{{err: err}}
	}
	var quoted []string
	for _, v := range splitCSV(val) {
		quoted = append(quoted, sqlident.QuoteLiteral(dialect, v))
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

func (relationshipCheck) Queries(_, schema, table, _, val string, execPlan *ExecutionPlan) []qualityQuery {
	// Parse "localCol to parentModel.parentCol" or "localCol->parentModel.parentCol"
	localCol, parentModel, parentCol, ok := parseRelationship(val)
	if !ok {
		return nil // malformed value — skip silently (matches prior behaviour)
	}
	if err := validateRelationship(val); err != nil {
		return []qualityQuery{{err: err}}
	}

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
