package virtual

import (
	"context"
	"fmt"

	"github.com/drover-org/drover-sqlforge/internal/state"
)

type Runner interface {
	Exec(ctx context.Context, sql string) error
}

type Manager struct {
	runner Runner
	state  *state.Manager
}

func NewManager(runner Runner, stateMgr *state.Manager) *Manager {
	return &Manager{runner: runner, state: stateMgr}
}

func (m *Manager) CreateVirtualEnv(ctx context.Context, envName, baseEnv string) error {
	env, err := m.state.GetOrCreateEnv(envName, baseEnv)
	if err != nil {
		return err
	}

	if err := m.runner.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", env.Schema)); err != nil {
		return err
	}

	return nil
}

func (m *Manager) Exec(ctx context.Context, sql string) error {
	return m.runner.Exec(ctx, sql)
}
