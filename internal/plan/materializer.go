package plan

import (
	"context"
	"fmt"
	"time"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

// ModelMaterializer applies a single model asset to the warehouse within an
// execution plan. It is constructed once per ApplyPlan call and shared across
// all worker goroutines. All fields are read-only after construction, making
// concurrent Apply calls safe without additional locking.
type ModelMaterializer struct {
	execPlan  *ExecutionPlan
	allModels map[string]bool // all model names in scope (read-only)
	parser    *parser.Parser  // nil-safe: skips WASM transpilation when nil
	vMgr      *virtual.Manager
	stateMgr  *state.Manager
	events    chan<- ApplyEvent // nil → log to stdout
	total     int
}

func newModelMaterializer(
	execPlan *ExecutionPlan,
	allModels map[string]bool,
	p *parser.Parser,
	vMgr *virtual.Manager,
	stateMgr *state.Manager,
	events chan<- ApplyEvent,
	total int,
) *ModelMaterializer {
	return &ModelMaterializer{
		execPlan:  execPlan,
		allModels: allModels,
		parser:    p,
		vMgr:      vMgr,
		stateMgr:  stateMgr,
		events:    events,
		total:     total,
	}
}

// Apply materialises one model asset against the warehouse. It is the single
// testable unit for the entire "compile → route → execute → verify → persist"
// pipeline. The steps are, in order:
//
//  1. Emit START event (or log to stdout).
//  2. Resolve the fully-qualified target (db, schema, table).
//  3. Build a dependency map and substitute structural references via the tokenizer.
//  4. Optionally transpile SQL across dialects via WASM.
//  5. Route the materialisation type to the correct DDL generator.
//  6. Execute the DDL.
//  7. Run data quality assertions.
//  8. Persist the model fingerprint to state.
//  9. Emit SUCCESS event (or return error with ERROR event).
func (m *ModelMaterializer) Apply(ctx context.Context, a *model.Asset) error {
	// 1. Progress signal
	if m.events != nil {
		m.events <- ApplyEvent{ModelName: a.Name, Type: EventStart}
	} else {
		fmt.Printf("[0/%d] Applying changes to %s...\n", m.total, a.Name)
	}

	matType := a.Config["materialized"]
	if matType == "" {
		matType = "view"
	}

	// 2. Resolve fully-qualified target identifiers
	targetDB, targetSchema, targetTable := ResolveTarget(m.execPlan.Environment.Schema, a)
	if targetDB != "" {
		targetSchema = targetDB + "." + targetSchema
	}

	// 3. Build dep map and substitute structural references (tokenizer)
	depMap := make(map[string]string)
	for _, dep := range a.Dependencies {
		if !m.allModels[dep] {
			continue
		}
		// Locate the dep asset across all plan buckets
		var depAsset *model.Asset
		allAssets := append(m.execPlan.ChangedModels,
			append(m.execPlan.Impacted, m.execPlan.Unchanged...)...)
		for _, pA := range allAssets {
			if pA.Name == dep {
				depAsset = pA
				break
			}
		}
		if depAsset != nil {
			dDB, dSchema, dTable := ResolveTarget(m.execPlan.Environment.Schema, depAsset)
			if dDB != "" {
				dSchema = dDB + "." + dSchema
			}
			depMap[dep] = dSchema + "." + dTable
		} else {
			depMap[dep] = m.execPlan.Environment.Schema + "." + dep // fallback
		}
	}
	compiledSQL := parser.ReplaceDependencies(a.SQL, depMap)

	// 4. WASM dialect transpilation (nil-safe)
	fromDialect := a.Config["dialect"]
	if fromDialect == "" {
		fromDialect = "ansi"
	}
	toDialect := m.vMgr.Runner().Name()
	if fromDialect != toDialect && m.parser != nil {
		if res, err := m.parser.TranspileWASM(compiledSQL, fromDialect, toDialect); err == nil && res.Error == "" {
			compiledSQL = res.SQL
		}
	}

	// 5. Route materialisation type → DDL
	ddl, err := m.buildDDL(ctx, a, matType, targetSchema, targetTable, compiledSQL)
	if err != nil {
		return m.fail(a.Name, err)
	}

	// 6. Execute DDL
	if err := m.vMgr.Exec(ctx, ddl); err != nil {
		return m.fail(a.Name, fmt.Errorf("failed to execute model %s: %w", a.Name, err))
	}

	// 7. Data quality assertions
	if err := RunDataQualityTestsFromConfig(ctx, m.vMgr.Runner(), a.Config, targetSchema, targetTable, m.execPlan); err != nil {
		return m.fail(a.Name, err)
	}

	// 8. Persist fingerprint
	fp, _ := graph.GenerateFingerprint(a.AST, a.Config)
	if err := m.stateMgr.Store.SaveModelState(&state.ModelState{
		ModelName:      a.Name,
		Fingerprint:    fp,
		LastApplied:    time.Now(),
		MaterializedAs: matType,
		Environment:    m.execPlan.Environment.Name,
	}); err != nil {
		return m.fail(a.Name, err)
	}

	// 9. Success
	if m.events != nil {
		m.events <- ApplyEvent{ModelName: a.Name, Type: EventSuccess}
	}
	return nil
}

// buildDDL routes the materialisation type to the correct DDL generator on the
// runner. It is a pure function of the materialisation type and the runner
// interface — no I/O except for the incremental TableExists probe.
func (m *ModelMaterializer) buildDDL(
	ctx context.Context,
	a *model.Asset,
	matType, targetSchema, targetTable, compiledSQL string,
) (string, error) {
	switch matType {
	case "incremental":
		exists, err := m.vMgr.Runner().TableExists(ctx, targetSchema, targetTable)
		if err != nil {
			return "", fmt.Errorf("failed to check table existence %s: %w", a.Name, err)
		}
		if !exists {
			return virtual.BuildIncrementalInitialDDL(
				m.vMgr.Runner().Name(), targetSchema, targetTable, compiledSQL, a.Config,
			)
		}
		return virtual.BuildIncrementalMergeDDL(
			m.vMgr.Runner().Name(), targetSchema, targetTable, compiledSQL, a.Config,
		)

	case "table":
		return m.vMgr.Runner().CreateTableDDL(targetSchema, targetTable, compiledSQL), nil

	case "materialized_view":
		return m.vMgr.Runner().CreateMaterializedViewDDL(targetSchema, targetTable, compiledSQL), nil

	case "kafka", "nats", "streaming":
		a.Config["_materialization_type"] = matType
		return m.vMgr.Runner().CreateStreamingTableDDL(targetSchema, targetTable, a.Config), nil

	default: // "view" and anything unrecognised
		return m.vMgr.Runner().CreateViewDDL(targetSchema, targetTable, compiledSQL), nil
	}
}

// fail emits an ERROR event (if channel is set) and returns the error.
func (m *ModelMaterializer) fail(modelName string, err error) error {
	if m.events != nil {
		m.events <- ApplyEvent{ModelName: modelName, Type: EventError, Error: err}
	}
	return err
}
