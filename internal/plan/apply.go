package plan

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
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

func loadSeeds(ctx context.Context, schema string, runner virtual.Runner) error {
	seedsDir := "seeds"
	if _, err := os.Stat(seedsDir); os.IsNotExist(err) {
		return nil
	}

	files, err := os.ReadDir(seedsDir)
	if err != nil {
		return nil
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		tableName := strings.TrimSuffix(file.Name(), ".csv")
		filePath := filepath.Join(seedsDir, file.Name())

		f, err := os.Open(filePath)
		if err != nil {
			return err
		}

		r := csv.NewReader(f)
		records, err := r.ReadAll()
		f.Close()
		if err != nil {
			return err
		}

		if len(records) == 0 {
			continue
		}

		headers := records[0]

		var colDefs []string
		for _, h := range headers {
			h = strings.TrimSpace(h)
			colDefs = append(colDefs, fmt.Sprintf("%s TEXT", h))
		}

		dropDDL := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s CASCADE", schema, tableName)
		_ = runner.Exec(ctx, dropDDL)

		createDDL := fmt.Sprintf("CREATE TABLE %s.%s (%s)", schema, tableName, strings.Join(colDefs, ", "))
		if err := runner.Exec(ctx, createDDL); err != nil {
			return fmt.Errorf("failed to create seed table %s: %w", tableName, err)
		}

		for _, row := range records[1:] {
			var vals []string
			for _, val := range row {
				val = strings.ReplaceAll(val, "'", "''")
				vals = append(vals, fmt.Sprintf("'%s'", val))
			}
			insertSQL := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)", 
				schema, tableName, strings.Join(headers, ", "), strings.Join(vals, ", "))
			if err := runner.Exec(ctx, insertSQL); err != nil {
				return fmt.Errorf("failed to insert seed row: %w", err)
			}
		}
	}
	return nil
}

func ApplyPlan(ctx context.Context, p *ExecutionPlan, stateMgr *state.Manager, vMgr *virtual.Manager, eventChan chan<- ApplyEvent) error {
	if eventChan == nil {
		fmt.Printf("Applying plan to environment: %s\n", p.Environment.Name)
	}

	if err := vMgr.CreateVirtualEnv(ctx, p.Environment.Name, p.Environment.BaseEnv); err != nil {
		return fmt.Errorf("failed to create virtual env: %w", err)
	}

	schema := p.Environment.Schema // e.g. "sqlforge__peter_dev"

	if vMgr.Runner().Name() == "postgres" {
		setSearchPath := fmt.Sprintf("SET search_path TO %s, public", schema)
		if err := vMgr.Exec(ctx, setSearchPath); err != nil {
			return fmt.Errorf("failed to set search path to %s: %w", schema, err)
		}
	}

	if err := loadSeeds(ctx, schema, vMgr.Runner()); err != nil {
		return fmt.Errorf("failed to load seeds: %w", err)
	}

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

		targetDB, targetSchema, targetTable := resolveTarget(p.Environment.Schema, a)
		if targetDB != "" {
			targetSchema = targetDB + "." + targetSchema
		}

		// Look up all dependencies and resolve their FQNs
		depMap := make(map[string]string)
		for _, dep := range a.Dependencies {
			if allModels[dep] {
				// We need the asset for the dependency to resolve its target.
				// For now, we can search the execution plan assets for it.
				var depAsset *model.Asset
				for _, pA := range append(p.ChangedModels, append(p.Impacted, p.Unchanged...)...) {
					if pA.Name == dep {
						depAsset = pA
						break
					}
				}
				
				if depAsset != nil {
					dDB, dSchema, dTable := resolveTarget(p.Environment.Schema, depAsset)
					if dDB != "" {
						dSchema = dDB + "." + dSchema
					}
					depMap[dep] = dSchema + "." + dTable
				} else {
					// Fallback
					depMap[dep] = p.Environment.Schema + "." + dep
				}
			}
		}
		transpiledSQL := parser.ReplaceDependencies(a.SQL, depMap)

		var ddl string
		if mat == "incremental" {
			exists, err := vMgr.Runner().TableExists(ctx, targetSchema, targetTable)
			if err != nil {
				if eventChan != nil {
					eventChan <- ApplyEvent{ModelName: a.Name, Type: EventError, Error: err}
				}
				return fmt.Errorf("failed to check table existence %s: %w", a.Name, err)
			}

			if !exists {
				var errDDL error
				ddl, errDDL = virtual.BuildIncrementalInitialDDL(vMgr.Runner().Name(), targetSchema, targetTable, transpiledSQL, a.Config)
				if errDDL != nil {
					return fmt.Errorf("incremental initial build for %s: %w", a.Name, errDDL)
				}
			} else {
				var errDDL error
				ddl, errDDL = virtual.BuildIncrementalMergeDDL(vMgr.Runner().Name(), targetSchema, targetTable, transpiledSQL, a.Config)
				if errDDL != nil {
					return fmt.Errorf("incremental merge for %s: %w", a.Name, errDDL)
				}
			}
		} else if mat == "table" {
			ddl = vMgr.Runner().CreateTableDDL(targetSchema, targetTable, transpiledSQL)
		} else if mat == "materialized_view" {
			ddl = vMgr.Runner().CreateMaterializedViewDDL(targetSchema, targetTable, transpiledSQL)
		} else if mat == "kafka" || mat == "nats" || mat == "streaming" {
			a.Config["_materialization_type"] = mat
			ddl = vMgr.Runner().CreateStreamingTableDDL(targetSchema, targetTable, a.Config)
		} else {
			ddl = vMgr.Runner().CreateViewDDL(targetSchema, targetTable, transpiledSQL)
		}

		// Execute the DDL against the live runner (or stub)
		if err := vMgr.Exec(ctx, ddl); err != nil {
			if eventChan != nil {
				eventChan <- ApplyEvent{ModelName: a.Name, Type: EventError, Error: err}
			}
			return fmt.Errorf("failed to execute model %s: %w", a.Name, err)
		}

		if err := RunDataQualityTests(ctx, vMgr.Runner(), a, targetSchema, targetTable); err != nil {
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

func resolveTarget(envSchema string, a *model.Asset) (db string, schema string, table string) {
	db = a.Config["database"]
	
	schema = envSchema
	if customSchema, ok := a.Config["schema"]; ok && customSchema != "" {
		schema = schema + "_" + customSchema
	}

	table = a.Name
	if customAlias, ok := a.Config["alias"]; ok && customAlias != "" {
		table = customAlias
	}
	return db, schema, table
}

func RunDataQualityTests(ctx context.Context, runner virtual.Runner, a *model.Asset, schema string, table string) error {
	for k, v := range a.Config {
		if k == "test_not_null" {
			cols := strings.Split(v, ",")
			for _, col := range cols {
				col = strings.TrimSpace(col)
				testSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE %s IS NULL", schema, table, col)
				count, err := runner.QueryCount(ctx, testSQL)
				if err != nil {
					return err
				}
				if count > 0 {
					return fmt.Errorf("data quality test failed: %s.%s.%s is not_null but found %d null records", schema, table, col, count)
				}
			}
		} else if k == "test_unique" {
			cols := strings.Split(v, ",")
			for _, col := range cols {
				col = strings.TrimSpace(col)
				testSQL := fmt.Sprintf("SELECT COUNT(*) FROM (SELECT %s, COUNT(*) as _c FROM %s.%s GROUP BY %s HAVING _c > 1)", col, schema, table, col)
				count, err := runner.QueryCount(ctx, testSQL)
				if err != nil {
					return err
				}
				if count > 0 {
					return fmt.Errorf("data quality test failed: %s.%s.%s is unique but found %d duplicate records", schema, table, col, count)
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
			testSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE %s NOT IN (%s)", schema, table, col, inClause)
			count, err := runner.QueryCount(ctx, testSQL)
			if err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("data quality test failed: %s.%s.%s contains %d records not in accepted values (%s)", schema, table, col, count, inClause)
			}
		}
	}
	return nil
}
