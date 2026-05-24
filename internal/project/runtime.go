package project

import (
	"context"
	"fmt"
	"os"

	"path/filepath"

	"github.com/drover-org/drover-sqlforge/internal/config"
	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/drover-org/drover-sqlforge/internal/plan"
	"github.com/drover-org/drover-sqlforge/internal/semantic"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

// Runtime holds a loaded data project ready for plan/apply/MCP operations.
type Runtime struct {
	ProjectDir string
	Parser     *parser.Parser
	StateMgr   *state.Manager
	VMgr       *virtual.Manager
	Env        *state.Environment
	Assets     []*model.Asset
	DAG        *graph.DAG
	Semantic   *semantic.Graph
}

// LoadRuntime loads config, parser, models, DAG, and environment for projectDir.
func LoadRuntime(projectDir, envName string) (*Runtime, error) {
	ctx := context.Background()

	p, err := parser.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("parser: %w", err)
	}

	stateMgr, err := state.NewManager(projectDir)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("state: %w", err)
	}

	cfg, err := config.LoadConfig(projectDir)
	var runner virtual.Runner
	if err != nil {
		runner, _ = virtual.NewRunner("clickhouse", "")
	} else {
		runner, err = virtual.NewRunner(cfg.Virtual.Dialect, cfg.Virtual.Connection)
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("runner: %w", err)
		}
	}

	vMgr := virtual.NewManager(runner, stateMgr)

	assets, err := model.LoadModels(filepath.Join(projectDir, "models"), p)
	if err != nil && !os.IsNotExist(err) {
		p.Close()
		return nil, fmt.Errorf("load models: %w", err)
	}
	if assets == nil {
		assets = make([]*model.Asset, 0)
	}

	packagesDir := filepath.Join(projectDir, "sqlforge_packages")
	entries, _ := os.ReadDir(packagesDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkgName := entry.Name()
		pkgDir := filepath.Join(packagesDir, pkgName)
		if pkgAssets, err := model.LoadModels(filepath.Join(pkgDir, "models"), p); err == nil {
			for _, a := range pkgAssets {
				if s, ok := a.Config["schema"]; ok && s != "" {
					a.Config["schema"] = pkgName + "_" + s
				} else {
					if a.Config == nil {
						a.Config = make(map[string]string)
					}
					a.Config["schema"] = pkgName
				}
			}
			assets = append(assets, pkgAssets...)
		}
	}

	baseEnv := "prod"
	if cfg != nil && cfg.DefaultEnvironment != "" {
		baseEnv = cfg.DefaultEnvironment
	}
	env, err := stateMgr.GetOrCreateEnv(envName, baseEnv)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("environment: %w", err)
	}

	semGraph, _ := semantic.LoadMetrics(projectDir)
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkgDir := filepath.Join(packagesDir, entry.Name())
		pkgGraph, _ := semantic.LoadMetrics(pkgDir)
		if pkgGraph != nil {
			if semGraph == nil {
				semGraph = pkgGraph
			} else {
				semGraph.Metrics = append(semGraph.Metrics, pkgGraph.Metrics...)
			}
		}
	}

	if semGraph != nil {
		compiler := semantic.NewCompiler("")
		for _, m := range semGraph.Metrics {
			if m.Materialize {
				sql, err := compiler.Compile(&m, m.Dimensions)
				if err == nil {
					assets = append(assets, &model.Asset{
						Name:         "semantic__" + m.Name,
						SQL:          sql,
						Config:       map[string]string{"materialized": "view"},
						Dependencies: []string{m.Model},
					})
				}
			}
		}
	}

	dag := graph.NewDAG()
	if err := dag.Build(assets); err != nil {
		p.Close()
		return nil, fmt.Errorf("dag: %w", err)
	}

	return &Runtime{
		ProjectDir: projectDir,
		Parser:     p,
		StateMgr:   stateMgr,
		VMgr:       vMgr,
		Env:        env,
		Assets:     assets,
		DAG:        dag,
		Semantic:   semGraph,
	}, nil
}

func (r *Runtime) Close() {
	if r.Parser != nil {
		r.Parser.Close()
	}
}

// ExecutionPlan builds the current execution plan for the environment.
func (r *Runtime) ExecutionPlan() (*plan.ExecutionPlan, error) {
	return plan.GeneratePlan(r.Env, r.Assets, r.StateMgr, r.DAG)
}

// EnsureEnvironment creates the warehouse schema for the bound environment.
func (r *Runtime) EnsureEnvironment(ctx context.Context) error {
	return r.VMgr.CreateVirtualEnv(ctx, r.Env.Name, r.Env.BaseEnv)
}
