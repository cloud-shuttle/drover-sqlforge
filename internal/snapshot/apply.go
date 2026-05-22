package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

// Result describes one snapshot execution.
type Result struct {
	Name    string
	Initial bool
	Error   error
}

// Apply runs historized snapshots in the given environment.
func Apply(ctx context.Context, defs []*Definition, env *state.Environment, vMgr *virtual.Manager, stateMgr *state.Manager) ([]Result, error) {
	if err := vMgr.CreateVirtualEnv(ctx, env.Name, env.BaseEnv); err != nil {
		return nil, fmt.Errorf("ensure environment: %w", err)
	}

	runner := vMgr.Runner()
	schema := env.Schema
	var results []Result

	for _, def := range defs {
		res := Result{Name: def.Name}
		cfg, err := ResolveConfig(def)
		if err != nil {
			res.Error = err
			results = append(results, res)
			continue
		}

		exists, err := runner.TableExists(ctx, schema, def.Name)
		if err != nil {
			res.Error = fmt.Errorf("table exists check: %w", err)
			results = append(results, res)
			continue
		}
		// No prior snapshot state => initial build even if a stale table remains (e.g. stub runners).
		if _, stateErr := stateMgr.Store.GetSnapshotState(def.Name, env.Name); stateErr != nil {
			exists = false
		}
		res.Initial = !exists

		stmts, err := BuildRun(runner.Name(), schema, def.Name, exists, def.SQL, cfg)
		if err != nil {
			res.Error = err
			results = append(results, res)
			continue
		}

		for _, stmt := range stmts {
			if err := vMgr.Exec(ctx, stmt); err != nil {
				res.Error = fmt.Errorf("execute snapshot SQL: %w", err)
				break
			}
		}
		if res.Error != nil {
			results = append(results, res)
			continue
		}

		fp := Fingerprint(def.SQL, def.Config)
		if err := stateMgr.Store.SaveSnapshotState(&state.SnapshotState{
			SnapshotName: def.Name,
			Environment:  env.Name,
			Fingerprint:  fp,
			LastApplied:  time.Now(),
			Strategy:     cfg.Strategy,
		}); err != nil {
			res.Error = fmt.Errorf("save state: %w", err)
		}
		results = append(results, res)
	}

	return results, nil
}
