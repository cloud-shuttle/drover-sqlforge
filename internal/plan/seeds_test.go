package plan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
)

func TestApplyPlanLoadsSeedCSV(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("seeds", 0o755); err != nil {
		t.Fatal(err)
	}
	body := "id,name\n1,O'Brien\n"
	if err := os.WriteFile(filepath.Join("seeds", "users.csv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	stateMgr, err := state.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := &MockRunner{}
	vMgr := newMockVMgr(runner, stateMgr)
	execPlan := &ExecutionPlan{
		Environment: &state.Environment{Name: "prod", Schema: "sqlforge__prod"},
	}
	if err := ApplyPlan(context.Background(), execPlan, stateMgr, vMgr, nil, nil, 1); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(runner.ExecLog, "\n")
	for _, want := range []string{
		"DROP TABLE IF EXISTS sqlforge__prod.users",
		"CREATE TABLE sqlforge__prod.users",
		"INSERT INTO sqlforge__prod.users",
		"O''Brien",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}
