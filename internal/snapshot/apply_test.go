package snapshot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
	"github.com/cloud-shuttle/drover-sqlforge/internal/virtual"
)

// stubRunner records SQL. TableExists stays true so a snapshot with no saved
// state is still treated as an initial build.
type stubRunner struct {
	sqls    []string
	execErr error
}

func (r *stubRunner) Exec(_ context.Context, sql string) error {
	r.sqls = append(r.sqls, sql)
	if r.execErr != nil && strings.Contains(sql, "CREATE TABLE") {
		return r.execErr
	}
	return nil
}
func (r *stubRunner) CreateSchemaDDL(schema string) string { return "CREATE SCHEMA " + schema }
func (r *stubRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return "table " + table
}
func (r *stubRunner) CreateViewDDL(schema, table, selectSQL string) string { return "view " + table }
func (r *stubRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return "mview " + table
}
func (r *stubRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return "stream " + table
}
func (r *stubRunner) TableExists(context.Context, string, string) (bool, error) { return true, nil }
func (r *stubRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	return "merge " + table
}
func (r *stubRunner) QueryCount(context.Context, string) (int, error) { return 0, nil }
func (r *stubRunner) QueryData(context.Context, string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (r *stubRunner) Name() string { return "postgres" }

func testEnv() *state.Environment {
	return &state.Environment{Name: "prod", Schema: "sqlforge__prod"}
}

func usersSnapshot() *Definition {
	return &Definition{
		Name: "users",
		SQL:  "SELECT 1 AS id, CURRENT_TIMESTAMP AS updated_at",
		Config: map[string]string{
			"strategy":   "timestamp",
			"unique_key": "id",
			"updated_at": "updated_at",
		},
	}
}

func TestApplyInitialThenIncremental(t *testing.T) {
	ctx := context.Background()
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := &stubRunner{}
	vMgr := virtual.NewManager(runner, stateMgr)
	def := usersSnapshot()

	first, err := Apply(ctx, []*Definition{def}, testEnv(), vMgr, stateMgr)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Error != nil || !first[0].Initial {
		t.Fatalf("first apply: %+v", first)
	}
	saved, err := stateMgr.Store.GetSnapshotState("users", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Fingerprint != Fingerprint(def.SQL, def.Config) || saved.Strategy != "timestamp" {
		t.Fatalf("saved state: %+v", saved)
	}

	second, err := Apply(ctx, []*Definition{def}, testEnv(), vMgr, stateMgr)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Error != nil || second[0].Initial {
		t.Fatalf("second apply: %+v", second)
	}
	joined := strings.Join(runner.sqls, "\n")
	if !strings.Contains(joined, "UPDATE") || !strings.Contains(joined, "INSERT INTO") {
		t.Fatalf("incremental SQL missing:\n%s", joined)
	}
}

func TestApplyContinuesAfterResolveError(t *testing.T) {
	ctx := context.Background()
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := &stubRunner{}
	vMgr := virtual.NewManager(runner, stateMgr)
	bad := &Definition{Name: "bad", SQL: "SELECT 1", Config: map[string]string{}}
	results, err := Apply(ctx, []*Definition{bad, usersSnapshot()}, testEnv(), vMgr, stateMgr)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results: %+v", results)
	}
	if results[0].Error == nil || !strings.Contains(results[0].Error.Error(), "unique_key") {
		t.Fatalf("bad result: %+v", results[0])
	}
	if results[1].Error != nil || !results[1].Initial {
		t.Fatalf("good result: %+v", results[1])
	}
}

func TestApplyExecErrorDoesNotSaveState(t *testing.T) {
	ctx := context.Background()
	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := &stubRunner{execErr: errors.New("boom")}
	vMgr := virtual.NewManager(runner, stateMgr)
	results, err := Apply(ctx, []*Definition{usersSnapshot()}, testEnv(), vMgr, stateMgr)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Error == nil || !strings.Contains(results[0].Error.Error(), "execute snapshot SQL") {
		t.Fatalf("result: %+v", results)
	}
	if _, err := stateMgr.Store.GetSnapshotState("users", "prod"); err == nil {
		t.Fatal("state was saved after a failed exec")
	}
}
