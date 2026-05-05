package plan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

type EventType string

const (
	EventStart   EventType = "START"
	EventSuccess EventType = "SUCCESS"
	EventError   EventType = "ERROR"
)

type ApplyEvent struct {
	ModelName string
	Type      EventType
	Error     error
}

func ApplyPlan(ctx context.Context, p *ExecutionPlan, stateMgr *state.Manager, vMgr *virtual.Manager, eventChan chan<- ApplyEvent) error {
	if eventChan == nil {
		fmt.Printf("Applying plan to environment: %s\n", p.Environment.Name)
	}

	// Ensure the database/schema exists
	if err := vMgr.CreateVirtualEnv(ctx, p.Environment.Name, p.Environment.BaseEnv); err != nil {
		return fmt.Errorf("failed to create virtual env: %w", err)
	}

	schema := p.Environment.Schema // e.g. "sqlforge__peter_dev"

	allModels := make(map[string]bool)
	for _, a := range p.ChangedModels {
		allModels[a.Name] = true
	}
	for _, a := range p.Impacted {
		allModels[a.Name] = true
	}
	for _, a := range p.Unchanged {
		allModels[a.Name] = true
	}

	applyModel := func(a *model.Asset, i, total int) error {
		if eventChan != nil {
			eventChan <- ApplyEvent{ModelName: a.Name, Type: EventStart}
		} else {
			fmt.Printf("[%d/%d] Applying changes to %s...\n", i, total, a.Name)
		}

		mat := a.Config["materialized"]
		if mat == "" {
			mat = "view"
		}

		// Transpile step: Replace table dependencies with their schema-prefixed versions.
		// We use a robust token-aware replacer to avoid accidental regex matches inside strings or aliases.
		depMap := make(map[string]string)
		for _, dep := range a.Dependencies {
			if allModels[dep] {
				depMap[dep] = schema + "." + dep
			}
		}
		transpiledSQL := parser.ReplaceDependencies(a.SQL, depMap)

		var ddl string
		if mat == "incremental" {
			exists, err := vMgr.Runner().TableExists(ctx, schema, a.Name)
			if err != nil {
				if eventChan != nil {
					eventChan <- ApplyEvent{ModelName: a.Name, Type: EventError, Error: err}
				}
				return fmt.Errorf("failed to check table existence %s: %w", a.Name, err)
			}

			if !exists {
				ddl = vMgr.Runner().CreateTableDDL(schema, a.Name, transpiledSQL)
			} else {
				ddl = vMgr.Runner().CreateIncrementalMergeDDL(schema, a.Name, transpiledSQL, a.Config)
			}
		} else if mat == "table" {
			ddl = vMgr.Runner().CreateTableDDL(schema, a.Name, transpiledSQL)
		} else if mat == "materialized_view" {
			ddl = vMgr.Runner().CreateMaterializedViewDDL(schema, a.Name, transpiledSQL)
		} else if mat == "kafka" || mat == "nats" || mat == "streaming" {
			a.Config["_materialization_type"] = mat
			ddl = vMgr.Runner().CreateStreamingTableDDL(schema, a.Name, a.Config)
		} else {
			ddl = vMgr.Runner().CreateViewDDL(schema, a.Name, transpiledSQL)
		}

		// Execute the DDL against the live runner (or stub)
		if err := vMgr.Exec(ctx, ddl); err != nil {
			if eventChan != nil {
				eventChan <- ApplyEvent{ModelName: a.Name, Type: EventError, Error: err}
			}
			return fmt.Errorf("failed to execute model %s: %w", a.Name, err)
		}

		if err := RunDataQualityTests(ctx, vMgr.Runner(), a, schema); err != nil {
			if eventChan != nil {
				eventChan <- ApplyEvent{ModelName: a.Name, Type: EventError, Error: err}
			}
			return err
		}

		fp, _ := graph.GenerateFingerprint(a.AST, a.Config)
		modelState := &state.ModelState{
			ModelName:      a.Name,
			Fingerprint:    fp,
			LastApplied:    time.Now(),
			MaterializedAs: mat,
			Environment:    p.Environment.Name,
		}

		err := stateMgr.Store.SaveModelState(modelState)
		if err != nil {
			if eventChan != nil {
				eventChan <- ApplyEvent{ModelName: a.Name, Type: EventError, Error: err}
			}
			return err
		}

		if eventChan != nil {
			eventChan <- ApplyEvent{ModelName: a.Name, Type: EventSuccess}
		}
		return nil
	}

	total := len(p.ChangedModels) + len(p.Impacted)

	for i, a := range p.ChangedModels {
		if err := applyModel(a, i+1, total); err != nil {
			return err
		}
	}

	for i, a := range p.Impacted {
		if err := applyModel(a, i+1+len(p.ChangedModels), total); err != nil {
			return err
		}
	}

	if eventChan == nil {
		fmt.Println("Apply completed successfully.")
	}
	return nil
}

func RunDataQualityTests(ctx context.Context, runner virtual.Runner, a *model.Asset, schema string) error {
	for k, v := range a.Config {
		if k == "test_not_null" {
			cols := strings.Split(v, ",")
			for _, col := range cols {
				col = strings.TrimSpace(col)
				testSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE %s IS NULL", schema, a.Name, col)
				count, err := runner.QueryCount(ctx, testSQL)
				if err != nil {
					return err
				}
				if count > 0 {
					return fmt.Errorf("data quality test failed: %s.%s.%s is not_null but found %d null records", schema, a.Name, col, count)
				}
			}
		} else if k == "test_unique" {
			cols := strings.Split(v, ",")
			for _, col := range cols {
				col = strings.TrimSpace(col)
				testSQL := fmt.Sprintf("SELECT COUNT(*) FROM (SELECT %s, COUNT(*) as _c FROM %s.%s GROUP BY %s HAVING _c > 1)", col, schema, a.Name, col)
				count, err := runner.QueryCount(ctx, testSQL)
				if err != nil {
					return err
				}
				if count > 0 {
					return fmt.Errorf("data quality test failed: %s.%s.%s is unique but found %d duplicate records", schema, a.Name, col, count)
				}
			}
		} else if strings.HasPrefix(k, "test_accepted_values_") {
			col := strings.TrimPrefix(k, "test_accepted_values_")
			vals := strings.Split(v, ",")
			var quotedVals []string
			for _, val := range vals {
				val = strings.TrimSpace(val)
				quotedVals = append(quotedVals, fmt.Sprintf("'%s'", val))
			}
			inClause := strings.Join(quotedVals, ", ")
			testSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE %s NOT IN (%s)", schema, a.Name, col, inClause)
			count, err := runner.QueryCount(ctx, testSQL)
			if err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("data quality test failed: %s.%s.%s contains %d records not in accepted values (%s)", schema, a.Name, col, count, inClause)
			}
		}
	}
	return nil
}
