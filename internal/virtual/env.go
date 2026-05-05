package virtual

import (
	"context"

	"github.com/drover-org/drover-sqlforge/internal/state"
)

type Runner interface {
	Exec(ctx context.Context, sql string) error
	CreateSchemaDDL(schema string) string
	CreateTableDDL(schema, table, selectSQL string) string
	CreateViewDDL(schema, table, selectSQL string) string
	CreateMaterializedViewDDL(schema, table, selectSQL string) string
	CreateStreamingTableDDL(schema, table string, config map[string]string) string
	TableExists(ctx context.Context, schema, table string) (bool, error)
	CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string
	QueryCount(ctx context.Context, sql string) (int, error)
	Name() string
}

type Manager struct {
	runner Runner
	state  *state.Manager
}

func NewManager(runner Runner, stateMgr *state.Manager) *Manager {
	return &Manager{runner: runner, state: stateMgr}
}

func (m *Manager) Runner() Runner {
	return m.runner
}

func (m *Manager) CreateVirtualEnv(ctx context.Context, envName, baseEnv string) error {
	env, err := m.state.GetOrCreateEnv(envName, baseEnv)
	if err != nil {
		return err
	}

	if err := m.runner.Exec(ctx, m.runner.CreateSchemaDDL(env.Schema)); err != nil {
		return err
	}

	return nil
}

func (m *Manager) Exec(ctx context.Context, sql string) error {
	return m.runner.Exec(ctx, sql)
}
