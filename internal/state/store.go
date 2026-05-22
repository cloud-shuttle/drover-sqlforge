package state

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func NewStore(projectPath string) (*Store, error) {
	dbPath := filepath.Join(projectPath, ".sqlforge", "state.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS environments (
			name TEXT PRIMARY KEY,
			schema TEXT,
			created_at DATETIME,
			is_virtual BOOLEAN,
			base_env TEXT
		);
		CREATE TABLE IF NOT EXISTS model_states (
			model_name TEXT,
			environment TEXT,
			fingerprint TEXT,
			last_applied DATETIME,
			materialized_as TEXT,
			PRIMARY KEY (model_name, environment)
		);
		CREATE TABLE IF NOT EXISTS snapshot_states (
			snapshot_name TEXT,
			environment TEXT,
			fingerprint TEXT,
			last_applied DATETIME,
			strategy TEXT,
			PRIMARY KEY (snapshot_name, environment)
		);
	`)

	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) GetEnvironment(name string) (*Environment, error) {
	row := s.db.QueryRow("SELECT name, schema, created_at, is_virtual, base_env FROM environments WHERE name = ?", name)
	env := &Environment{}
	if err := row.Scan(&env.Name, &env.Schema, &env.CreatedAt, &env.IsVirtual, &env.BaseEnv); err != nil {
		return nil, err
	}
	return env, nil
}

func (s *Store) SaveEnvironment(env *Environment) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO environments (name, schema, created_at, is_virtual, base_env)
		VALUES (?, ?, ?, ?, ?)`,
		env.Name, env.Schema, env.CreatedAt, env.IsVirtual, env.BaseEnv)
	return err
}

func (s *Store) GetModelState(modelName, env string) (*ModelState, error) {
	row := s.db.QueryRow("SELECT model_name, environment, fingerprint, last_applied, materialized_as FROM model_states WHERE model_name = ? AND environment = ?", modelName, env)
	state := &ModelState{}
	if err := row.Scan(&state.ModelName, &state.Environment, &state.Fingerprint, &state.LastApplied, &state.MaterializedAs); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *Store) SaveModelState(state *ModelState) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO model_states (model_name, environment, fingerprint, last_applied, materialized_as)
		VALUES (?, ?, ?, ?, ?)`,
		state.ModelName, state.Environment, state.Fingerprint, state.LastApplied, state.MaterializedAs)
	return err
}

func (s *Store) GetSnapshotState(snapshotName, env string) (*SnapshotState, error) {
	row := s.db.QueryRow(
		"SELECT snapshot_name, environment, fingerprint, last_applied, strategy FROM snapshot_states WHERE snapshot_name = ? AND environment = ?",
		snapshotName, env,
	)
	st := &SnapshotState{}
	if err := row.Scan(&st.SnapshotName, &st.Environment, &st.Fingerprint, &st.LastApplied, &st.Strategy); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) SaveSnapshotState(state *SnapshotState) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO snapshot_states (snapshot_name, environment, fingerprint, last_applied, strategy)
		VALUES (?, ?, ?, ?, ?)`,
		state.SnapshotName, state.Environment, state.Fingerprint, state.LastApplied, state.Strategy)
	return err
}
