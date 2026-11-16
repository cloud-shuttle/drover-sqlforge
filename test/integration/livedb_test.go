package integration

// Live-database tests for runner SQL (#58). They copy the fixture project in
// livedb/, point it at a real database, and run the released CLI through
// apply / test / incremental re-apply, then read the results back with a
// database driver. Hostile environment, model and config names must be
// rejected without executing anything (canary table survives).
//
//   - Postgres: runs when SQLFORGE_TEST_POSTGRES_DSN is set (CI starts a
//     postgres:16 service container); skipped otherwise.
//   - DuckDB: always runs, in-process via the DuckDB plugin and a temp file.
//     Needs CGO (the plugin and the test's driver link libduckdb).

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/marcboeker/go-duckdb"
)

const rawTable = "sqlforge_it_raw_events"
const canaryTable = "sqlforge_it_canary"

var (
	buildOnce sync.Once
	binDir    string
	buildErr  error
)

// buildBinaries compiles sqlforge and the DuckDB plugin once per test run.
func buildBinaries(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		binDir, buildErr = os.MkdirTemp("", "sqlforge-livedb-bin-*")
		if buildErr != nil {
			return
		}
		for out, pkg := range map[string]string{
			"sqlforge":               "./cmd/sqlforge",
			"sqlforge-plugin-duckdb": "./cmd/plugins/sqlforge-plugin-duckdb",
		} {
			cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, out), pkg)
			cmd.Dir = root
			if b, err := cmd.CombinedOutput(); err != nil {
				buildErr = fmt.Errorf("go build %s: %v\n%s", pkg, err, b)
				return
			}
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return binDir
}

// liveDB abstracts the warehouse under test. For DuckDB, every call opens and
// closes the file because the plugin needs the lock while the CLI runs.
type liveDB struct {
	dialect string
	dsn     string // as written into sqlforge.yml
	open    func() (*sql.DB, error)
}

func (d liveDB) exec(t *testing.T, stmts ...string) {
	t.Helper()
	db, err := d.open()
	if err != nil {
		t.Fatalf("open %s: %v", d.dialect, err)
	}
	defer db.Close()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

func (d liveDB) queryInt(t *testing.T, q string) int {
	t.Helper()
	db, err := d.open()
	if err != nil {
		t.Fatalf("open %s: %v", d.dialect, err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
	return n
}

func (d liveDB) tableExists(t *testing.T, schema, table string) bool {
	t.Helper()
	return d.queryInt(t, fmt.Sprintf(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '%s' AND table_name = '%s'",
		schema, table)) > 0
}

type liveProject struct {
	t   *testing.T
	dir string
}

func newLiveProject(t *testing.T, db liveDB) liveProject {
	t.Helper()
	bin := buildBinaries(t)
	dir := t.TempDir()
	if err := copyDir(filepath.Join("livedb"), dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"sqlforge", "sqlforge-plugin-duckdb"} {
		data, err := os.ReadFile(filepath.Join(bin, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := fmt.Sprintf("name: livedb\nversion: 1.0.0\ndefault_environment: prod\nvirtual:\n  dialect: %s\n  connection: %q\n", db.dialect, db.dsn)
	if err := os.WriteFile(filepath.Join(dir, "sqlforge.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return liveProject{t: t, dir: dir}
}

func (p liveProject) run(args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(p.dir, "sqlforge"), args...)
	cmd.Dir = p.dir
	cmd.Env = append(os.Environ(), "TERM=dumb")
	out, err := cmd.CombinedOutput()
	reapPlugins(p.dir)
	return string(out), err
}

// reapPlugins kills plugin processes the CLI left behind in dir. On the
// current base the CLI does not stop its go-plugin children on exit, and a
// leaked DuckDB plugin keeps the database file locked for the next command.
// #73 (#59) fixes the leak; delete this helper once it has merged.
func reapPlugins(dir string) {
	if runtime.GOOS != "linux" {
		return
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return
	}
	procs, _ := os.ReadDir("/proc")
	for _, e := range procs {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil || !strings.HasPrefix(string(comm), "sqlforge-plugin") {
			continue
		}
		cwd, err := os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
		if err != nil || cwd != want {
			continue
		}
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
			_, _ = proc.Wait()
		}
	}
	// Give the kernel a moment to release the file lock.
	time.Sleep(100 * time.Millisecond)
}

func (p liveProject) mustRun(args ...string) string {
	p.t.Helper()
	out, err := p.run(args...)
	if err != nil {
		p.t.Fatalf("sqlforge %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (p liveProject) writeModel(name, body string) {
	p.t.Helper()
	if err := os.WriteFile(filepath.Join(p.dir, "models", name), []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func uniqueEnv(dialect string) string {
	return fmt.Sprintf("it_%s_%d", dialect, time.Now().UnixNano()%1_000_000_000)
}

// runLiveSuite is the dialect-independent scenario.
func runLiveSuite(t *testing.T, db liveDB) {
	db.exec(t,
		"DROP TABLE IF EXISTS "+rawTable,
		"CREATE TABLE "+rawTable+" (id INTEGER, amount INTEGER, updated_at TIMESTAMP)",
		"INSERT INTO "+rawTable+" VALUES (1, 10, TIMESTAMP '2026-01-01 00:00:00'), (2, 20, TIMESTAMP '2026-01-01 00:00:00'), (3, 30, TIMESTAMP '2026-01-01 00:00:00')",
		"DROP TABLE IF EXISTS "+canaryTable,
		"CREATE TABLE "+canaryTable+" (id INTEGER)",
	)
	env := uniqueEnv(db.dialect)
	schema := "sqlforge__" + env

	t.Run("apply builds table, view and incremental models", func(t *testing.T) {
		p := newLiveProject(t, db)
		out := p.mustRun("apply", env)
		if !strings.Contains(out, "Apply completed successfully.") {
			t.Fatalf("apply output:\n%s", out)
		}
		for _, m := range []string{"stg_events", "inc_merge", "inc_delete_insert", "inc_append"} {
			if n := db.queryInt(t, "SELECT COUNT(*) FROM "+schema+"."+m); n != 3 {
				t.Errorf("%s: %d rows, want 3", m, n)
			}
		}
		if n := db.queryInt(t, "SELECT SUM(doubled) FROM "+schema+".events_view"); n != 120 {
			t.Errorf("events_view SUM(doubled) = %d, want 120", n)
		}

		out = p.mustRun("test", env)
		if !strings.Contains(out, "3 passed, 0 failed") {
			t.Errorf("sqlforge test:\n%s", out)
		}

		// Source changes: id 1 updated, id 4 added. Re-apply each incremental
		// model so the second run takes the merge path (table exists).
		db.exec(t,
			"UPDATE "+rawTable+" SET amount = 99 WHERE id = 1",
			"INSERT INTO "+rawTable+" VALUES (4, 40, TIMESTAMP '2026-01-02 00:00:00')",
		)
		for _, m := range []string{"inc_merge", "inc_delete_insert", "inc_append"} {
			p.mustRun("apply", env, "--model", m)
		}
		for _, m := range []string{"inc_merge", "inc_delete_insert"} {
			if n := db.queryInt(t, "SELECT COUNT(*) FROM "+schema+"."+m); n != 4 {
				t.Errorf("%s after merge: %d rows, want 4 (no duplicates)", m, n)
			}
			if n := db.queryInt(t, "SELECT amount FROM "+schema+"."+m+" WHERE id = 1"); n != 99 {
				t.Errorf("%s after merge: id 1 amount = %d, want 99", m, n)
			}
		}
		if n := db.queryInt(t, "SELECT COUNT(*) FROM "+schema+".inc_append"); n != 7 {
			t.Errorf("inc_append after append: %d rows, want 3+4", n)
		}
	})

	t.Run("hostile environment name never executes", func(t *testing.T) {
		p := newLiveProject(t, db)
		hostile := "x;DROP TABLE " + canaryTable + ";--"
		out, err := p.run("apply", hostile)
		if err == nil {
			t.Fatalf("apply %q succeeded:\n%s", hostile, out)
		}
		if !strings.Contains(out, "invalid environment name") {
			t.Errorf("expected an invalid environment name error, got:\n%s", out)
		}
		if !db.tableExists(t, db.defaultSchema(), canaryTable) {
			t.Fatal("canary table was dropped")
		}
	})

	t.Run("hostile model file name never executes", func(t *testing.T) {
		p := newLiveProject(t, db)
		p.writeModel("x;DROP TABLE "+canaryTable+";--.sql", "-- @materialized: table\nSELECT 1 AS id\n")
		env2 := uniqueEnv(db.dialect)
		out, err := p.run("apply", env2)
		if err == nil {
			t.Fatalf("apply with hostile model name succeeded:\n%s", out)
		}
		if !db.tableExists(t, db.defaultSchema(), canaryTable) {
			t.Fatal("canary table was dropped")
		}
		if db.tableExists(t, "sqlforge__"+env2, "stg_events") {
			t.Error("other models were built even though the project was rejected")
		}
	})

	t.Run("hostile unique_key never executes", func(t *testing.T) {
		p := newLiveProject(t, db)
		p.writeModel("inc_merge.sql", "-- @materialized: incremental\n-- @unique_key: id); DROP TABLE "+canaryTable+"; --\nSELECT id, amount, updated_at FROM "+rawTable+"\n")
		env3 := uniqueEnv(db.dialect)
		out, err := p.run("apply", env3)
		if err == nil {
			t.Fatalf("apply with hostile unique_key succeeded:\n%s", out)
		}
		if !db.tableExists(t, db.defaultSchema(), canaryTable) {
			t.Fatal("canary table was dropped")
		}
		if db.tableExists(t, "sqlforge__"+env3, "stg_events") {
			t.Error("models were built before the hostile config was rejected")
		}
	})
}

func (d liveDB) defaultSchema() string {
	if d.dialect == "duckdb" {
		return "main"
	}
	return "public"
}

func TestLivePostgres(t *testing.T) {
	dsn := os.Getenv("SQLFORGE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SQLFORGE_TEST_POSTGRES_DSN not set (CI runs this against a postgres service container)")
	}
	runLiveSuite(t, liveDB{
		dialect: "postgres",
		dsn:     dsn,
		open:    func() (*sql.DB, error) { return sql.Open("postgres", dsn) },
	})
}

func TestLiveDuckDB(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "live.duckdb")
	runLiveSuite(t, liveDB{
		dialect: "duckdb",
		dsn:     file,
		open:    func() (*sql.DB, error) { return sql.Open("duckdb", file) },
	})
}
