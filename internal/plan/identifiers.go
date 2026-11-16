package plan

import (
	"fmt"
	"strings"

	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlident"
)

// ValidateApplyTargets checks every identifier that ApplyPlan will
// interpolate into generated SQL (environment schema, per-model database,
// schema and table, and column names from model config) before anything is
// executed. Values in model config come from Git, but they are still treated
// as untrusted input to SQL text.
func ValidateApplyTargets(execPlan *ExecutionPlan) error {
	if execPlan == nil || execPlan.Environment == nil {
		return nil
	}
	if err := sqlident.ValidateIdentifier("environment schema", execPlan.Environment.Schema); err != nil {
		return err
	}
	all := append(append(append([]*model.Asset{}, execPlan.ChangedModels...), execPlan.Impacted...), execPlan.Unchanged...)
	for _, a := range all {
		if err := ValidateModelTarget(execPlan.Environment.Schema, a); err != nil {
			return err
		}
	}
	return nil
}

// ValidateModelTarget validates the resolved target and the config-supplied
// column names of one model.
func ValidateModelTarget(envSchema string, a *model.Asset) error {
	if a == nil {
		return nil
	}
	db, schema, table := ResolveTarget(envSchema, a)
	if db != "" {
		if err := sqlident.ValidateIdentifier("database", db); err != nil {
			return fmt.Errorf("model %s: %w", a.Name, err)
		}
	}
	if err := sqlident.ValidateIdentifier("schema", schema); err != nil {
		return fmt.Errorf("model %s: %w", a.Name, err)
	}
	if err := sqlident.ValidateIdentifier("table", table); err != nil {
		return fmt.Errorf("model %s: %w", a.Name, err)
	}
	return validateModelColumns(a)
}

func validateModelColumns(a *model.Asset) error {
	for k, v := range a.Config {
		var err error
		switch {
		case k == "unique_key" && strings.TrimSpace(v) != "":
			err = sqlident.ValidateColumnList("unique_key", v)
		case k == "updated_at" && strings.TrimSpace(v) != "":
			err = sqlident.ValidateIdentifier("updated_at column", strings.TrimSpace(v))
		case k == "test_not_null" || k == "test_unique":
			err = sqlident.ValidateColumnList(k+" column", v)
		case strings.HasPrefix(k, "test_accepted_values_"):
			err = sqlident.ValidateIdentifier("accepted_values column", strings.TrimPrefix(k, "test_accepted_values_"))
		case strings.HasPrefix(k, "test_relationship"):
			err = validateRelationship(v)
		}
		if err != nil {
			return fmt.Errorf("model %s: config %s: %w", a.Name, k, err)
		}
	}
	return nil
}

// parseRelationship parses "localCol to parentModel.parentCol" (or "->").
func parseRelationship(val string) (localCol, parentModel, parentCol string, ok bool) {
	parts := strings.Split(val, " to ")
	if len(parts) != 2 {
		parts = strings.Split(val, "->")
	}
	if len(parts) != 2 {
		return "", "", "", false
	}
	parentParts := strings.Split(strings.TrimSpace(parts[1]), ".")
	if len(parentParts) != 2 {
		return "", "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parentParts[0]), strings.TrimSpace(parentParts[1]), true
}

func validateRelationship(val string) error {
	local, parent, parentCol, ok := parseRelationship(val)
	if !ok {
		return nil // malformed values are skipped by the check itself
	}
	for kind, name := range map[string]string{"relationship column": local, "relationship model": parent, "relationship parent column": parentCol} {
		if err := sqlident.ValidateIdentifier(kind, name); err != nil {
			return err
		}
	}
	return nil
}
