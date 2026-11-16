//go:build smoke

// Package smoke is the consumer-facing end-to-end check (#59): it builds the
// released binaries (sqlforge + the DuckDB plugin), copies the bundled example
// project into a temp dir, points it at a DuckDB file, and runs it through
// plan → apply → test → query → docs generate exactly as a new user would,
// then opens the DuckDB file to assert what was actually built.
//
// Run with `make smoke` (or `go test -tags smoke ./test/smoke/`). Needs CGO
// and a C++ toolchain for the DuckDB plugin.
package smoke

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

const env = "prod"

// Tables the example builds and their row counts. The staging models
// generate fixed-size data; -1 means "non-empty" for models grouped by
// now()-relative dates, whose row counts depend on the time of day.
var wantRows = map[string]int{
	"sqlforge__prod_staging.stg_users":                  100,
	"sqlforge__prod_staging.stg_events":                 1000,
	"sqlforge__prod_staging.stg_experiments":            100,
	"sqlforge__prod_intermediate.int_customer_sessions": 200,
	"sqlforge__prod_marts.customer_360":                 100,
	"sqlforge__prod_marts.daily_metrics":                1000,
	"sqlforge__prod_marts.ab_test_results":              -1,
	"sqlforge__prod.semantic__daily_active_users":       -1,
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func build(t *testing.T, root, out, pkg string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, b)
	}
}

func copyDir(t *testing.T, src, dst string, skip func(rel string) bool) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel != "." && skip(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

type project struct {
	t   *testing.T
	dir string
}

// run executes ./sqlforge in the project and returns combined output.
func (p project) run(wantOK bool, args ...string) string {
	p.t.Helper()
	cmd := exec.Command(filepath.Join(p.dir, "sqlforge"), args...)
	cmd.Dir = p.dir
	// Non-TTY output path; quiet go-plugin logs.
	cmd.Env = append(os.Environ(), "TERM=dumb", "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if wantOK && err != nil {
		p.t.Fatalf("sqlforge %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func mustContain(t *testing.T, step, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Fatalf("%s: output missing %q\n---\n%s", step, w, out)
		}
	}
}

func leakedPlugins(t *testing.T, dir string) int {
	t.Helper()
	out, _ := exec.Command("pgrep", "-f", filepath.Join(dir, "sqlforge-plugin-")).Output()
	out2, _ := exec.Command("pgrep", "-f", "^./sqlforge-plugin-duckdb").Output()
	n := 0
	for _, b := range [][]byte{out, out2} {
		for _, l := range strings.Fields(string(b)) {
			if l != "" {
				n++
			}
		}
	}
	return n
}

func TestConsumerSmoke(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}
	root := repoRoot(t)
	dir := t.TempDir()

	copyDir(t, filepath.Join(root, "examples", "agentic_retail_2026"), dir, func(rel string) bool {
		base := filepath.Base(rel)
		// Local state and stray DuckDB files from earlier manual runs.
		return base == ".sqlforge" || base == "memory" || base == "target" || strings.HasPrefix(base, "duckdb.db") || strings.HasSuffix(base, ".duckdb")
	})
	cfgPath := filepath.Join(dir, "sqlforge.yml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "connection: memory") {
		t.Fatalf("example sqlforge.yml no longer uses `connection: memory`; update the smoke test")
	}
	cfg = []byte(strings.Replace(string(cfg), "connection: memory", "connection: smoke.duckdb", 1))
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	build(t, root, filepath.Join(dir, "sqlforge"), "./cmd/sqlforge")
	build(t, root, filepath.Join(dir, "sqlforge-plugin-duckdb"), "./cmd/plugins/sqlforge-plugin-duckdb")
	p := project{t: t, dir: dir}

	// 1. plan: a fresh project plans every model.
	out := p.run(true, "plan", env)
	mustContain(t, "plan", out, "Changed Models: 8")

	// 2. apply
	out = p.run(true, "apply", env)
	mustContain(t, "apply", out, "All singular tests passed", "Apply completed successfully.")

	// 3. test: a separate process must see the data apply wrote.
	out = p.run(true, "test", env)
	mustContain(t, "test", out, "6 passed, 0 failed")

	// 4. plan again: nothing left to do.
	out = p.run(true, "plan", env)
	mustContain(t, "re-plan", out, "Changed Models: 0", "Impacted Models: 0")

	// 5. query: compile metrics through the semantic layer. The printed SQL
	// must run as-is against the warehouse (checked in step 7).
	out = p.run(true, "query", "daily_active_users", env, "--dimensions", "country")
	mustContain(t, "query daily_active_users", out, "Compiled ANSI SQL for 'daily_active_users'", "FROM sqlforge__prod_marts.daily_metrics")
	dauSQL := between(out, "---\n", "\n---")
	out = p.run(true, "query", "revenue_per_user", env, "--dimensions", "variant")
	mustContain(t, "query revenue_per_user", out, "FROM sqlforge__prod_marts.ab_test_results")
	rpuSQL := between(out, "---\n", "\n---")
	if dauSQL == "" || rpuSQL == "" {
		t.Fatalf("query: could not extract compiled SQL\n%s", out)
	}

	// 6. docs generate
	p.run(true, "docs", "generate", env)
	html, err := os.ReadFile(filepath.Join(dir, "target", "index.html"))
	if err != nil {
		t.Fatalf("docs generate: %v", err)
	}
	for _, w := range []string{"window.SQLFORGE_CATALOG", "customer_360", "daily_active_users"} {
		if !strings.Contains(string(html), w) {
			t.Fatalf("docs: index.html missing %q", w)
		}
	}

	// Every CLI run must stop its plugin, or DuckDB stays locked.
	if n := leakedPlugins(t, dir); n != 0 {
		t.Fatalf("%d sqlforge-plugin process(es) still running after the CLI exited", n)
	}

	// 7. Inspect the warehouse directly.
	db, err := sql.Open("duckdb", filepath.Join(dir, "smoke.duckdb")+"?access_mode=read_only")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for model, want := range wantRows {
		var got int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + model).Scan(&got); err != nil {
			t.Errorf("count %s: %v", model, err)
			continue
		}
		switch {
		case want >= 0 && got != want:
			t.Errorf("%s: %d rows, want %d", model, got, want)
		case want < 0 && got == 0:
			t.Errorf("%s: empty", model)
		}
	}

	// daily_active_users by country: the example only has US users.
	var country string
	var dau int
	if err := db.QueryRow(strings.TrimSuffix(dauSQL, ";")).Scan(&country, &dau); err != nil {
		t.Fatalf("execute compiled daily_active_users SQL: %v\n%s", err, dauSQL)
	}
	if country != "US" || dau <= 0 || dau > 100 {
		t.Fatalf("daily_active_users = %s/%d, want US with 1..100 users", country, dau)
	}

	// revenue_per_user by variant: one non-null row per variant.
	rows, err := db.Query(rpuSQL)
	if err != nil {
		t.Fatalf("execute compiled revenue_per_user SQL: %v\n%s", err, rpuSQL)
	}
	defer rows.Close()
	variants := 0
	for rows.Next() {
		var variant string
		var rpu sql.NullFloat64
		if err := rows.Scan(&variant, &rpu); err != nil {
			t.Fatal(err)
		}
		if !rpu.Valid || rpu.Float64 < 0 {
			t.Errorf("revenue_per_user[%s] = %v, want a non-negative number", variant, rpu)
		}
		variants++
	}
	if variants == 0 {
		t.Fatal("revenue_per_user returned no rows")
	}
	t.Logf("daily_active_users[US]=%d, revenue_per_user variants=%d", dau, variants)
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}
