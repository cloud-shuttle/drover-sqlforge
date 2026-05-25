package plan

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

	schema := execPlan.Environment.Schema

	if vMgr.Runner().Name() == "postgres" {
		setSearchPath := fmt.Sprintf("SET search_path TO %s, public", schema)
		if err := vMgr.Exec(ctx, setSearchPath); err != nil {
			return fmt.Errorf("failed to set search path to %s: %w", schema, err)
		}
	}

	if err := loadSeeds(ctx, schema, vMgr.Runner()); err != nil {
		return fmt.Errorf("failed to load seeds: %w", err)
	}

	// Ensure all target schemas exist sequentially before parallel execution
	// (avoids catalog write-write conflicts on single-writer warehouses like DuckDB).
	createdSchemas := make(map[string]bool)
	for _, a := range append(execPlan.ChangedModels, execPlan.Impacted...) {
		_, targetSchema, _ := ResolveTarget(execPlan.Environment.Schema, a)
		if !createdSchemas[targetSchema] {
			if schemaDDL := vMgr.Runner().CreateSchemaDDL(targetSchema); schemaDDL != "" {
				if err := vMgr.Exec(ctx, schemaDDL); err != nil {
					return fmt.Errorf("failed to ensure schema %s exists: %w", targetSchema, err)
				}
			}
			createdSchemas[targetSchema] = true
		}
	}

	// Build the full model name set used for dependency resolution.
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

	total := len(execPlan.ChangedModels) + len(execPlan.Impacted)
	if total == 0 {
		return nil
	}

	mat := newModelMaterializer(execPlan, allModels, p, vMgr, stateMgr, eventChan, total)

	// Build in-degree map and out-edges for topological scheduling.
	inDegree := make(map[string]int)
	outEdges := make(map[string][]string)
	planModels := make(map[string]*model.Asset)

	for _, a := range append(execPlan.ChangedModels, execPlan.Impacted...) {
		planModels[a.Name] = a
		inDegree[a.Name] = 0
	}
	for _, a := range planModels {
		for _, dep := range a.Dependencies {
			if _, exists := planModels[dep]; exists {
				inDegree[a.Name]++
				outEdges[dep] = append(outEdges[dep], a.Name)
			}
		}
	}

	// Seed the ready channel with zero-in-degree nodes.
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
		go func() {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case a := <-readyChan:
					if err := mat.Apply(workerCtx, a); err != nil {
						errChan <- err
						return
					}
					doneChan <- a.Name
				}
			}
		}()
	}

	var mu sync.Mutex
	completed := 0
	var firstErr error

	for completed < total {
		select {
		case err := <-errChan:
			if firstErr == nil {
				firstErr = err
				cancel()
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

	cancel()
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	// Run singular tests (tests/ directory) after all models are applied.
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

func ResolveTarget(envSchema string, a *model.Asset) (db string, schema string, table string) {
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

func ResolveParentTarget(envSchema string, parentModel string, execPlan *ExecutionPlan) string {
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
		db, schema, table := ResolveTarget(envSchema, targetAsset)
		if db != "" {
			return db + "." + schema + "." + table
		}
		return schema + "." + table
	}

	return envSchema + "." + parentModel
}

// RunDataQualityTests is the original exported entry point retained for
// backwards compatibility with existing test and MCP callers. It delegates
// to RunDataQualityTestsFromConfig in quality_check.go.
func RunDataQualityTests(ctx context.Context, runner virtual.Runner, a *model.Asset, schema string, table string, execPlan *ExecutionPlan) error {
	return RunDataQualityTestsFromConfig(ctx, runner, a.Config, schema, table, execPlan)
}

func RunSingularTests(ctx context.Context, runner virtual.Runner, tests []*model.Asset, execPlan *ExecutionPlan, p *parser.Parser) error {
	for _, test := range tests {
		depMap := make(map[string]string)
		for _, dep := range test.Dependencies {
			var envSchema string
			if execPlan != nil {
				envSchema = execPlan.Environment.Schema
			}
			depMap[dep] = ResolveParentTarget(envSchema, dep, execPlan)
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

		sqlToNest := strings.TrimSpace(transpiledSQL)
		if strings.HasSuffix(sqlToNest, ";") {
			sqlToNest = strings.TrimSuffix(sqlToNest, ";")
		}

		testSQL := fmt.Sprintf("SELECT COUNT(*) FROM (\n%s\n) AS _test_assertion", sqlToNest)

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
