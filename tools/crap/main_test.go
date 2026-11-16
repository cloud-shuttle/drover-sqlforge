package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseFunc(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\n"+src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f.Decls[0].(*ast.FuncDecl)
}

func TestComplexity(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"func f() {}", 1},
		{"func f(a bool) { if a {} }", 2},
		{"func f(a, b bool) { if a && b || a {} }", 4},
		{"func f(xs []int) { for range xs {}; for i := 0; i < 1; i++ {} }", 3},
		{"func f(x int) { switch x { case 1: case 2, 3: default: } }", 3},
		{"func f(c chan int) { select { case <-c: default: } }", 2},
		{"func f(a bool) { g := func() { if a {} }; g() }", 2},
	}
	for _, c := range cases {
		if got := complexity(parseFunc(t, c.src)); got != c.want {
			t.Errorf("complexity(%q) = %d, want %d", c.src, got, c.want)
		}
	}
}

func TestFuncName(t *testing.T) {
	cases := map[string]string{
		"func f() {}":                  "f",
		"func (s *Server) Run() {}":    "(*Server).Run",
		"func (s Server) Run() {}":     "(Server).Run",
		"func (s *Set[T]) Add(v T) {}": "(*Set).Add",
	}
	for src, want := range cases {
		if got := funcName(parseFunc(t, src)); got != want {
			t.Errorf("funcName(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestCRAP(t *testing.T) {
	if got := crap(5, 1); got != 5 {
		t.Errorf("fully covered CRAP = %v, want complexity", got)
	}
	if got := crap(5, 0); got != 30 {
		t.Errorf("uncovered CRAP = %v, want c^2+c", got)
	}
	if got := crap(10, 0.5); math.Abs(got-22.5) > 1e-9 {
		t.Errorf("crap(10, .5) = %v", got)
	}
}

func TestParseProfileLineAndCoverage(t *testing.T) {
	file, b, _, ok := parseProfileLine("github.com/x/y/a.go:10.2,12.3 4 1")
	if !ok || file != "github.com/x/y/a.go" || b.startLine != 10 || b.endLine != 12 || b.stmts != 4 || b.count != 1 {
		t.Fatalf("parse = %q %+v %v", file, b, ok)
	}
	if _, _, _, ok := parseProfileLine("garbage"); ok {
		t.Fatal("garbage parsed")
	}
	blocks := []block{{10, 12, 4, 1}, {13, 14, 4, 0}, {30, 31, 9, 1}}
	if got := functionCoverage(blocks, 9, 20); got != 0.5 {
		t.Errorf("coverage = %v, want 0.5", got)
	}
	if got := functionCoverage(blocks, 40, 50); got != 0 {
		t.Errorf("no blocks coverage = %v, want 0", got)
	}
}

func TestEvaluateAndRatchet(t *testing.T) {
	scores := []fnScore{
		{Key: "a.go:new", CRAP: 31},     // new offender: fails at limit 30
		{Key: "a.go:legacy", CRAP: 100}, // pinned at 120: passes
		{Key: "a.go:worse", CRAP: 130},  // pinned at 120: fails
		{Key: "a.go:fixed", CRAP: 12},   // pinned but now under the limit
		{Key: "a.go:fine", CRAP: 2},
	}
	ceilings := map[string]float64{"a.go:legacy": 120, "a.go:worse": 120, "a.go:fixed": 200, "a.go:gone": 50}
	v := evaluate(scores, ceilings, 30)
	if len(v) != 2 || v[0].Key != "a.go:new" || v[1].Key != "a.go:worse" {
		t.Fatalf("violations = %+v", v)
	}

	next := ratchetCeilings(scores, ceilings, 30)
	want := map[string]float64{"a.go:legacy": 101, "a.go:worse": 120}
	if len(next) != len(want) {
		t.Fatalf("ratchet = %v, want %v", next, want)
	}
	for k, w := range want {
		if next[k] != w {
			t.Errorf("ratchet[%s] = %v, want %v", k, next[k], w)
		}
	}
	if _, ok := next["a.go:new"]; ok {
		t.Error("ratchet must not add new offenders")
	}

	init := initCeilings(scores, 30)
	if len(init) != 3 || init["a.go:new"] != 32 {
		t.Errorf("init = %v", init)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunEndToEnd drives the CLI over a tiny module: a covered simple
// function, and an uncovered branchy one (complexity 6, CRAP 42).
func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "a.go"), `package m

func Simple() int {
	return 1
}

func Branchy(a, b, c, d, e int) int {
	if a > 0 {
		return 1
	}
	if b > 0 {
		return 2
	}
	if c > 0 {
		return 3
	}
	if d > 0 {
		return 4
	}
	if e > 0 {
		return 5
	}
	return 0
}
`)
	writeFile(t, filepath.Join(dir, "a_test.go"), "package m\n")
	writeFile(t, filepath.Join(dir, "gen.pb.go"), "package m\nfunc G(a bool) { if a {}; if a {}; if a {}; if a {}; if a {}; if a {} }\n")
	writeFile(t, filepath.Join(dir, "coverage.out"), "mode: atomic\n"+
		"example.com/m/a.go:3.20,5.2 1 3\n"+
		"example.com/m/a.go:7.37,8.11 1 0\n"+
		"example.com/m/a.go:8.11,10.3 1 0\n")
	cov := filepath.Join(dir, "coverage.out")
	base := filepath.Join(dir, "baseline.json")
	runArgs := func(extra ...string) (int, string) {
		var out, errOut bytes.Buffer
		code := run(append([]string{"-root", dir, "-coverage", cov}, extra...), &out, &errOut)
		return code, out.String() + errOut.String()
	}

	if code, out := runArgs(); code != 1 || !strings.Contains(out, "a.go:Branchy: CRAP 42.00") {
		t.Fatalf("no baseline: code %d, out:\n%s", code, out)
	}
	if code, out := runArgs("-baseline", base); code != 2 {
		t.Fatalf("missing baseline without -init should be an error: code %d\n%s", code, out)
	}
	if code, out := runArgs("-baseline", base, "-init"); code != 0 || !strings.Contains(out, "wrote 1 ceilings") {
		t.Fatalf("init: code %d\n%s", code, out)
	}
	if code, out := runArgs("-baseline", base); code != 0 || strings.Contains(out, "gen.pb.go") {
		t.Fatalf("pinned baseline: code %d\n%s", code, out)
	}
	if code, out := runArgs("-baseline", base, "-update"); code != 0 || !strings.Contains(out, "wrote 1 ceilings") {
		t.Fatalf("update: code %d\n%s", code, out)
	}
	if code, out := runArgs("-baseline", base, "-limit", "50"); code != 0 {
		t.Fatalf("raised limit: code %d\n%s", code, out)
	}
	if code, _ := runArgs("-bogus"); code != 2 {
		t.Fatalf("bad flag: code %d", code)
	}
	if code, _ := runArgs("-coverage", filepath.Join(dir, "missing.out")); code != 2 {
		t.Fatalf("missing profile: code %d", code)
	}
}
