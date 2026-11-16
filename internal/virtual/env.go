package virtual

import (
	"io"
	"context"

	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
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
	QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error)
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

// Close releases the runner's resources, stopping its plugin process if it
// has one. Runners without resources to release are left alone.
func (m *Manager) Close() error {
	if m == nil || m.runner == nil {
		return nil
	}
	if c, ok := m.runner.(io.Closer); ok {
		return c.Close()
	}
	return nil
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
