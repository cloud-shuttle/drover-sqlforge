package plan

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
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

		// Transpile step (stub): replace known DAG dependencies with schema.table
		transpiledSQL := a.SQL
		for _, dep := range a.Dependencies {
			if allModels[dep] {
				re := regexp.MustCompile(`(?i)\b` + dep + `\b`)
				transpiledSQL = re.ReplaceAllString(transpiledSQL, schema+"."+dep)
			}
		}

		var ddl string
		if mat == "table" || mat == "incremental" {
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
