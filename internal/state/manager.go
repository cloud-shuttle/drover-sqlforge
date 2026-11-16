package state

import (
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlident"
)

type Manager struct {
	Store *Store
}

func NewManager(projectPath string) (*Manager, error) {
	store, err := NewStore(projectPath)
	if err != nil {
		return nil, err
	}
	return &Manager{Store: store}, nil
}

func (m *Manager) GetOrCreateEnv(name, baseEnv string) (*Environment, error) {
	// The environment name becomes part of a schema identifier in generated SQL.
	if err := sqlident.ValidateEnvName(name); err != nil {
		return nil, err
	}
	env, err := m.Store.GetEnvironment(name)
	if err == nil {
		return env, nil
	}
	newEnv := &Environment{
		Name:      name,
		Schema:    "sqlforge__" + name,
		IsVirtual: true,
		BaseEnv:   baseEnv,
		CreatedAt: time.Now(),
	}
	err = m.Store.SaveEnvironment(newEnv)
	return newEnv, err
}
