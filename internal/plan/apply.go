package plan

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func ApplyPlan(ctx context.Context, execPlan *ExecutionPlan, stateMgr *state.Manager, vMgr *virtual.Manager, p *parser.Parser, eventChan chan<- ApplyEvent, threads int) error {
	if eventChan == nil {
		fmt.Printf("Applying plan to environment: %s\n", execPlan.Environment.Name)
	}

	if err := vMgr.CreateVirtualEnv(ctx, execPlan.Environment.Name, execPlan.Environment.BaseEnv); err != nil {
		return fmt.Errorf("failed to create virtual env: %w", err)
	}

	schema := execPlan.Environment.Schema // e.g. "sqlforge__peter_dev"

	if vMgr.Runner().Name() == "postgres" {
		setSearchPath := fmt.Sprintf("SET search_path TO %s, public", schema)
		if err := vMgr.Exec(ctx, setSearchPath); err != nil {
			return fmt.Errorf("failed to set search path to %s: %w", schema, err)
		}
	}

	if err := loadSeeds(ctx, schema, vMgr.Runner()); err != nil {
		return fmt.Errorf("failed to load seeds: %w", err)
	}

	// Ensure all target schemas exist sequentially before parallel execution (avoids catalog write-write conflicts)
	createdSchemas := make(map[string]bool)
	for _, a := range append(execPlan.ChangedModels, execPlan.Impacted...) {
		_, targetSchema, _ := resolveTarget(execPlan.Environment.Schema, a)
		if !createdSchemas[targetSchema] {
			if schemaDDL := vMgr.Runner().CreateSchemaDDL(targetSchema); schemaDDL != "" {
				if err := vMgr.Exec(ctx, schemaDDL); err != nil {
					return fmt.Errorf("failed to ensure schema %s exists: %w", targetSchema, err)
				}
			}
			createdSchemas[targetSchema] = true
		}
	}

	allModels := make(map[string]bool)
	for _, a := range execPlan.ChangedModels {
		allModels[a.Name] = true
	}
	for _, a := range execPlan.Impacted {
		allModels[a.Name] = true
	}
	for _, a := range execPlan.Unchanged {
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

		targetDB, targetSchema, targetTable := resolveTarget(execPlan.Environment.Schema, a)
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
				for _, pA := range append(execPlan.ChangedModels, append(execPlan.Impacted, execPlan.Unchanged...)...) {
					if pA.Name == dep {
						depAsset = pA
						break
					}
				}
				
				if depAsset != nil {
					dDB, dSchema, dTable := resolveTarget(execPlan.Environment.Schema, depAsset)
					if dDB != "" {
						dSchema = dDB + "." + dSchema
					}
					depMap[dep] = dSchema + "." + dTable
				} else {
					// Fallback
					depMap[dep] = execPlan.Environment.Schema + "." + dep
				}
			}
		}
		transpiledSQL := parser.ReplaceDependencies(a.SQL, depMap)

		// Transpile Step: Translate SQL from source dialect to target environment dialect
		fromDialect := a.Config["dialect"]
		if fromDialect == "" {
			fromDialect = "ansi" // default or get from connection
		}
		toDialect := vMgr.Runner().Name()
		
		if fromDialect != "" && fromDialect != toDialect && p != nil {
			res, err := p.TranspileWASM(transpiledSQL, fromDialect, toDialect)
			if err == nil && res.Error == "" {
				transpiledSQL = res.SQL
			}
		}

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

		if err := RunDataQualityTests(ctx, vMgr.Runner(), a, targetSchema, targetTable, execPlan); err != nil {
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
			Environment:    execPlan.Environment.Name,
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

	total := len(execPlan.ChangedModels) + len(execPlan.Impacted)
	if total == 0 {
		return nil
	}

	// 1. Build dependency map (in-degree and out-edges)
	inDegree := make(map[string]int)
	outEdges := make(map[string][]string) // node -> dependents
	planModels := make(map[string]*model.Asset)

	for _, a := range append(execPlan.ChangedModels, execPlan.Impacted...) {
		planModels[a.Name] = a
		inDegree[a.Name] = 0 // Initialize all nodes in the plan
	}

	for _, a := range planModels {
		for _, dep := range a.Dependencies {
			if _, exists := planModels[dep]; exists {
				inDegree[a.Name]++
				outEdges[dep] = append(outEdges[dep], a.Name)
			}
		}
	}

	// 2. Setup Worker Pool
	readyChan := make(chan *model.Asset, total)
	errChan := make(chan error, total)
	doneChan := make(chan string, total)

	for name, count := range inDegree {
		if count == 0 {
			readyChan <- planModels[name]
		}
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	if threads < 1 {
		threads = 1
	}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case a := <-readyChan:
					if err := applyModel(a, 0, total); err != nil {
						errChan <- err
						return
					}
					doneChan <- a.Name
				}
			}
		}(i)
	}

	// 3. Orchestrate
	var mu sync.Mutex
	completed := 0
	var firstErr error

	for completed < total {
		select {
		case err := <-errChan:
			if firstErr == nil {
				firstErr = err
				cancel() // Stop all workers
			}
		case name := <-doneChan:
			completed++
			mu.Lock()
			for _, dependent := range outEdges[name] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					readyChan <- planModels[dependent]
				}
			}
			mu.Unlock()
		}
		if firstErr != nil {
			break
		}
	}

	cancel() // Ensure workers shut down
	wg.Wait() // Wait for them to finish current tasks

	if firstErr != nil {
		return firstErr
	}

	// Load and run singular tests if tests directory exists
	testsDir := "tests"
	if _, err := os.Stat(testsDir); err == nil {
		testAssets, err := model.LoadSingularTests(testsDir, p)
		if err == nil && len(testAssets) > 0 {
			if eventChan == nil {
				fmt.Printf("Running %d singular tests...\n", len(testAssets))
			}
			if err := RunSingularTests(ctx, vMgr.Runner(), testAssets, execPlan, p); err != nil {
				return err
			}
			if eventChan == nil {
				fmt.Println("All singular tests passed successfully.")
			}
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

func resolveParentTarget(envSchema string, parentModel string, execPlan *ExecutionPlan) string {
	if execPlan == nil {
		return envSchema + "." + parentModel
	}

	var targetAsset *model.Asset
	for _, a := range append(execPlan.ChangedModels, append(execPlan.Impacted, execPlan.Unchanged...)...) {
		if a.Name == parentModel {
			targetAsset = a
			break
		}
	}

	if targetAsset != nil {
		db, schema, table := resolveTarget(envSchema, targetAsset)
		if db != "" {
			return db + "." + schema + "." + table
		}
		return schema + "." + table
	}

	return envSchema + "." + parentModel
}

func RunDataQualityTests(ctx context.Context, runner virtual.Runner, a *model.Asset, schema string, table string, execPlan *ExecutionPlan) error {
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
		} else if strings.HasPrefix(k, "test_relationship") {
			parts := strings.Split(v, " to ")
			if len(parts) != 2 {
				parts = strings.Split(v, "->")
			}
			if len(parts) == 2 {
				localCol := strings.TrimSpace(parts[0])
				parentTarget := strings.TrimSpace(parts[1])

				parentParts := strings.Split(parentTarget, ".")
				if len(parentParts) == 2 {
					parentModel := strings.TrimSpace(parentParts[0])
					parentCol := strings.TrimSpace(parentParts[1])

					var envSchema string
					if execPlan != nil {
						envSchema = execPlan.Environment.Schema
					} else {
						envSchema = schema
					}

					parentTableFQN := resolveParentTarget(envSchema, parentModel, execPlan)

					testSQL := fmt.Sprintf(
						"SELECT COUNT(*) FROM %s.%s WHERE %s IS NOT NULL AND %s NOT IN (SELECT %s FROM %s)",
						schema, table, localCol, localCol, parentCol, parentTableFQN,
					)

					count, err := runner.QueryCount(ctx, testSQL)
					if err != nil {
						return err
					}
					if count > 0 {
						return fmt.Errorf("data quality test failed: relationship validation failed between %s.%s.%s and %s.%s (found %d invalid records)", schema, table, localCol, parentTableFQN, parentCol, count)
					}
				}
			}
		}
	}
	return nil
}

func RunSingularTests(ctx context.Context, runner virtual.Runner, tests []*model.Asset, execPlan *ExecutionPlan, p *parser.Parser) error {
	for _, test := range tests {
		depMap := make(map[string]string)
		for _, dep := range test.Dependencies {
			var envSchema string
			if execPlan != nil {
				envSchema = execPlan.Environment.Schema
			}
			depMap[dep] = resolveParentTarget(envSchema, dep, execPlan)
		}

		transpiledSQL := parser.ReplaceDependencies(test.SQL, depMap)

		fromDialect := test.Config["dialect"]
		if fromDialect == "" {
			fromDialect = "ansi"
		}
		toDialect := runner.Name()
		if fromDialect != "" && fromDialect != toDialect && p != nil {
			res, err := p.TranspileWASM(transpiledSQL, fromDialect, toDialect)
			if err == nil && res.Error == "" {
				transpiledSQL = res.SQL
			}
		}

		testSQL := fmt.Sprintf("SELECT COUNT(*) FROM (\n%s\n) AS _test_assertion", transpiledSQL)

		count, err := runner.QueryCount(ctx, testSQL)
		if err != nil {
			return fmt.Errorf("singular test %s failed to execute: %w", test.Name, err)
		}

		if count > 0 {
			return fmt.Errorf("data quality test failed: singular assertion %s returned %d failing records", test.Name, count)
		}
	}
	return nil
}
