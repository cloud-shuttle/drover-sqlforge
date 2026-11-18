package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/mcp"
	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
	"github.com/cloud-shuttle/drover-sqlforge/internal/plan"
	"github.com/cloud-shuttle/drover-sqlforge/internal/project"
	"github.com/cloud-shuttle/drover-sqlforge/internal/sqlguard"
	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
	"github.com/cloud-shuttle/drover-sqlforge/internal/virtual"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Warehouse tests apply test/integration/warehouse (view, table, incremental
// append, and incremental merge) through project.LoadRuntime and plan.ApplyPlan.
//
// Postgres starts via testcontainers and is skipped when docker is missing.
// DuckDB uses the sqlforge-plugin-duckdb binary and is skipped when that
// binary is missing or fails to start (it needs CGO; CI builds it).

func TestPostgresApplyViewTableIncrementalMerge(t *testing.T) {
	skipWithoutDocker(t)
	ctx := context.Background()
	dsn, cleanup := startPostgres(t, ctx)
	defer cleanup()
	runWarehouseApply(t, ctx, "postgres", dsn)
}

func TestDuckDBApplyViewTableIncrementalMerge(t *testing.T) {
	if !duckDBPluginPresent() {
		t.Skip("sqlforge-plugin-duckdb not found (run `make plugins`; CI builds it before go test)")
	}
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "warehouse.duckdb")
	runWarehouseApply(t, ctx, "duckdb", dsn)
}

func TestPostgresInjectionNeverExecutes(t *testing.T) {
	skipWithoutDocker(t)
	ctx := context.Background()
	dsn, cleanup := startPostgres(t, ctx)
	defer cleanup()

	admin, err := virtual.NewPostgresRunner(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	// Two statements, plus a quoted semicolon that must not become a third.
	script := "CREATE TABLE public.injection_sentinel (id int); INSERT INTO public.injection_sentinel VALUES (1); SELECT 'keep; DROP TABLE public.injection_sentinel'"
	if err := admin.Exec(ctx, script); err != nil {
		t.Fatalf("setup sentinel: %v", err)
	}
	if got := sentinelCount(t, ctx, admin); got != 1 {
		t.Fatalf("sentinel count = %d after setup", got)
	}

	dir := writeWarehouseProject(t, "postgres", dsn)
	rt, err := project.LoadRuntime(dir, "prod")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	counter := wrapRunner(rt)

	rejectPlanChange(t, ctx, rt)
	if counter.calls != 0 {
		t.Fatalf("plan_change executed %d statements: %q", counter.calls, counter.sqls)
	}
	if got := sentinelCount(t, ctx, admin); got != 1 {
		t.Fatalf("sentinel count = %d after plan_change", got)
	}

	rejectHostileApply(t, ctx, rt, counter)
	if got := sentinelCount(t, ctx, admin); got != 1 {
		t.Fatalf("sentinel count = %d after hostile apply", got)
	}

	_, err = rt.StateMgr.GetOrCreateEnv("prod; DROP TABLE public.injection_sentinel", "prod")
	if err == nil || !strings.Contains(err.Error(), "invalid environment") {
		t.Fatalf("hostile env err = %v", err)
	}
	if _, err := project.LoadRuntime(dir, "qa; DROP TABLE public.injection_sentinel"); err == nil || !strings.Contains(err.Error(), "invalid environment") {
		t.Fatalf("hostile LoadRuntime env err = %v", err)
	}

	badFile := filepath.Join(dir, "models", "evil;drop.sql")
	if err := os.WriteFile(badFile, []byte("SELECT 1 AS id;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LoadRuntime(dir, "prod"); err == nil || !strings.Contains(err.Error(), "invalid model") {
		t.Fatalf("hostile model file err = %v", err)
	}
	if counter.calls != 0 {
		t.Fatalf("executed %d statements: %q", counter.calls, counter.sqls)
	}
	if got := sentinelCount(t, ctx, admin); got != 1 {
		t.Fatalf("sentinel count = %d at end", got)
	}
}

func runWarehouseApply(t *testing.T, ctx context.Context, dialect, dsn string) {
	t.Helper()
	dir := writeWarehouseProject(t, dialect, dsn)
	rt, err := project.LoadRuntime(dir, "prod")
	if err != nil {
		if dialect == "duckdb" && pluginStartFailure(err) {
			t.Skipf("duckdb plugin unavailable: %v", err)
		}
		t.Fatalf("load: %v", err)
	}
	defer rt.Close()
	if got := rt.VMgr.Runner().Name(); got != dialect {
		t.Fatalf("runner %q, want %q", got, dialect)
	}
	setModelDialect(rt, dialect)

	applyRuntime(t, ctx, rt)
	assertInitialWarehouse(t, ctx, rt.VMgr.Runner())

	rewriteModel(t, rt, "users_incremental", "SELECT 1 AS id, 'inc2' AS name\nUNION ALL\nSELECT 2 AS id, 'inc2' AS name")
	rewriteModel(t, rt, "users_merge", "SELECT 1 AS id, 'm1b' AS name")
	applyRuntime(t, ctx, rt)
	assertIncrementalWarehouse(t, ctx, rt.VMgr.Runner())
}

func applyRuntime(t *testing.T, ctx context.Context, rt *project.Runtime) {
	t.Helper()
	execPlan, err := rt.ExecutionPlan()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	events := make(chan plan.ApplyEvent, 64)
	if err := plan.ApplyPlan(ctx, execPlan, rt.StateMgr, rt.VMgr, rt.Parser, events, 1); err != nil {
		t.Fatalf("apply: %v", err)
	}
}

func setModelDialect(rt *project.Runtime, dialect string) {
	for _, a := range rt.Assets {
		if a.Config == nil {
			a.Config = map[string]string{}
		}
		a.Config["dialect"] = dialect
	}
}

func rewriteModel(t *testing.T, rt *project.Runtime, name, sql string) {
	t.Helper()
	ast, err := rt.Parser.ParseToAST(sql)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rt.Assets {
		if a.Name != name {
			continue
		}
		a.SQL = sql
		a.AST = ast
		return
	}
	t.Fatalf("model %s not loaded", name)
}

func assertInitialWarehouse(t *testing.T, ctx context.Context, r virtual.Runner) {
	t.Helper()
	// QueryCount, not QueryData: the DuckDB plugin's gRPC client stubs QueryData.
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_view", 2)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_view WHERE id = 1 AND name = 'alice' AND note = 'keep;me'", 1)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_view WHERE id = 2 AND name = 'bob' AND note = 'plain'", 1)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.stg_users", 2)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_incremental WHERE name = 'inc'", 2)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_merge WHERE id = 1 AND name = 'm1'", 1)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_merge WHERE id = 3 AND name = 'm3'", 1)
}

func assertIncrementalWarehouse(t *testing.T, ctx context.Context, r virtual.Runner) {
	t.Helper()
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_incremental", 4)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_incremental WHERE name = 'inc'", 2)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_incremental WHERE name = 'inc2'", 2)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_merge", 2)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_merge WHERE id = 1 AND name = 'm1b'", 1)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_merge WHERE id = 3 AND name = 'm3'", 1)
	requireCount(t, ctx, r, "SELECT count(*) FROM sqlforge__prod.users_view WHERE id = 1 AND name = 'alice' AND note = 'keep;me'", 1)
}

func requireCount(t *testing.T, ctx context.Context, r virtual.Runner, query string, want int) {
	t.Helper()
	got, err := r.QueryCount(ctx, query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	if got != want {
		t.Fatalf("%s: count %d, want %d", query, got, want)
	}
}

func rejectPlanChange(t *testing.T, ctx context.Context, rt *project.Runtime) {
	t.Helper()
	store := mcp.NewPlanStore()
	reg := mcp.NewRegistry()
	reg.InitializeCoreTools(rt, store)
	tool, ok := reg.Get("plan_change")
	if !ok {
		t.Fatal("plan_change not registered")
	}
	payload := "SELECT 1 AS id; DROP TABLE public.injection_sentinel"
	params, err := json.Marshal(map[string]string{"model_name": "stg_users", "proposed_sql": payload})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tool.Handler(ctx, params)
	var ge *sqlguard.Error
	if !errors.As(err, &ge) {
		t.Fatalf("plan_change err = %v, want sqlguard rejection", err)
	}
}

func rejectHostileApply(t *testing.T, ctx context.Context, rt *project.Runtime, counter *countingRunner) {
	t.Helper()
	env := &state.Environment{Name: "prod", Schema: "sqlforge__prod", BaseEnv: "prod"}
	cases := []*plan.ExecutionPlan{
		{
			Environment: &state.Environment{Name: "prod", Schema: "sqlforge__prod; DROP TABLE public.injection_sentinel", BaseEnv: "prod"},
			ChangedModels: []*model.Asset{{
				Name: "stg_users", SQL: "SELECT 1 AS id", Config: map[string]string{"materialized": "table"},
			}},
		},
		{
			Environment: env,
			ChangedModels: []*model.Asset{{
				Name: "stg_users; DROP TABLE public.injection_sentinel", SQL: "SELECT 1 AS id",
				Config: map[string]string{"materialized": "table"},
			}},
		},
		{
			Environment: env,
			ChangedModels: []*model.Asset{{
				Name: "stg_users", SQL: "SELECT 1 AS id; DROP TABLE public.injection_sentinel",
				Config: map[string]string{"materialized": "table", "alias": "users; DROP TABLE public.injection_sentinel"},
			}},
		},
	}
	before := counter.calls
	for i, execPlan := range cases {
		err := plan.ApplyPlan(ctx, execPlan, rt.StateMgr, rt.VMgr, rt.Parser, make(chan plan.ApplyEvent, 8), 1)
		if err == nil {
			t.Fatalf("case %d: hostile apply succeeded", i)
		}
	}
	if counter.calls != before {
		t.Fatalf("hostile apply executed %q", counter.sqls[before:])
	}
}

type countingRunner struct {
	virtual.Runner
	calls int
	sqls  []string
}

func (c *countingRunner) Exec(ctx context.Context, sql string) error {
	c.calls++
	c.sqls = append(c.sqls, sql)
	return c.Runner.Exec(ctx, sql)
}

func (c *countingRunner) Close() error {
	if closer, ok := c.Runner.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func wrapRunner(rt *project.Runtime) *countingRunner {
	counter := &countingRunner{Runner: rt.VMgr.Runner()}
	rt.VMgr = virtual.NewManager(counter, rt.StateMgr)
	return counter
}

func sentinelCount(t *testing.T, ctx context.Context, r virtual.Runner) int {
	t.Helper()
	n, err := r.QueryCount(ctx, "SELECT count(*) FROM public.injection_sentinel")
	if err != nil {
		t.Fatalf("sentinel query: %v", err)
	}
	return n
}

func writeWarehouseProject(t *testing.T, dialect, dsn string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "project")
	if err := copyDir(warehouseFixture(t), dst); err != nil {
		t.Fatalf("copy project: %v", err)
	}
	yml := fmt.Sprintf("name: warehouse_integration\nversion: 1.0.0\ndefault_environment: prod\n\nvirtual:\n  dialect: %s\n  connection: %q\n", dialect, dsn)
	if err := os.WriteFile(filepath.Join(dst, "sqlforge.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func warehouseFixture(t *testing.T) string {
	t.Helper()
	if cwd, err := os.Getwd(); err == nil {
		src := filepath.Join(cwd, "warehouse")
		if st, err := os.Stat(src); err == nil && st.IsDir() {
			return src
		}
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "warehouse")
}

func startPostgres(t *testing.T, ctx context.Context) (string, func()) {
	t.Helper()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("sqlforge"),
		postgres.WithUsername("sqlforge"),
		postgres.WithPassword("sqlforge"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(context.Background())
		t.Fatalf("postgres dsn: %v", err)
	}
	return dsn, func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres: %v", err)
		}
	}
}

func skipWithoutDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker binary not found; skipping postgres integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Skipf("docker is not available (%v); skipping postgres integration test", err)
	}
}

func duckDBPluginPresent() bool {
	for _, name := range []string{
		"sqlforge-plugin-duckdb",
		filepath.Join("..", "..", "sqlforge-plugin-duckdb"),
		filepath.Join("..", "..", "bin", "sqlforge-plugin-duckdb"),
	} {
		st, err := os.Stat(name)
		if err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func pluginStartFailure(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "failed to start plugin") ||
		strings.Contains(msg, "failed to dispense") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file")
}
