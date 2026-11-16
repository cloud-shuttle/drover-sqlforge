// Command crap is the CI quality gate for CRAP (Change Risk Anti-Patterns)
// scores, computed per Go function:
//
//	CRAP(f) = complexity(f)^2 * (1 - coverage(f))^3 + complexity(f)
//
// complexity is cyclomatic complexity (1 + if, for, range, non-default case
// and select clauses, &&, ||), and coverage is the share of the function's
// statements hit in a Go coverage profile. A function fails when its score is
// above -limit, unless it is a legacy function pinned in the baseline file, in
// which case it fails only when it gets worse than its recorded ceiling.
//
// Usage:
//
//	go run ./tools/crap -coverage coverage.out -baseline quality/crap-baseline.json
//	go run ./tools/crap ... -update   # lower ceilings after adding tests
//	go run ./tools/crap ... -init     # regenerate the baseline from scratch
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultLimit = 30.0

type block struct {
	startLine, endLine int
	stmts              int
	count              int
}

type fnScore struct {
	Key        string
	Complexity int
	Coverage   float64
	CRAP       float64
	Allowed    float64
}

type baseline struct {
	Comment string             `json:"_comment"`
	Limit   float64            `json:"limit"`
	Funcs   map[string]float64 `json:"functions"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the gate and returns the process exit code: 0 pass, 1 CRAP
// violations, 2 usage or input errors.
func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("crap", flag.ContinueOnError)
	fl.SetOutput(stderr)
	root := fl.String("root", ".", "module root to scan")
	coverage := fl.String("coverage", "coverage.out", "Go coverage profile")
	basePath := fl.String("baseline", "", "JSON file with per-function ceilings for legacy functions")
	limitFlag := fl.Float64("limit", 0, "CRAP limit (default: the baseline's limit, else 30)")
	update := fl.Bool("update", false, "lower or drop baseline ceilings that improved, then exit 0 (never raises or adds)")
	initFlag := fl.Bool("init", false, "write a fresh baseline from this run, then exit 0")
	top := fl.Int("top", 15, "number of highest scores to print")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "crap: %v\n", err)
		return 2
	}

	base, err := loadBaseline(*basePath, *initFlag)
	if err != nil {
		return fail(err)
	}
	limit := base.Limit
	if *limitFlag > 0 {
		limit = *limitFlag
	}

	module, err := modulePath(filepath.Join(*root, "go.mod"))
	if err != nil {
		return fail(err)
	}
	blocks, err := readProfile(*coverage, module)
	if err != nil {
		return fail(err)
	}
	scores, err := scoreFunctions(*root, blocks)
	if err != nil {
		return fail(err)
	}

	if *initFlag || *update {
		funcs := initCeilings(scores, limit)
		if *update {
			funcs = ratchetCeilings(scores, base.Funcs, limit)
		}
		if err := writeBaseline(*basePath, limit, funcs); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "wrote %d ceilings to %s\n", len(funcs), *basePath)
		return 0
	}

	violations := evaluate(scores, base.Funcs, limit)
	report(stdout, scores, violations, limit, len(base.Funcs), *top)
	if len(violations) > 0 {
		return 1
	}
	return 0
}

// loadBaseline reads the ceilings file. A missing file is only accepted with
// -init (or when no path is given, which means "no legacy ceilings").
func loadBaseline(path string, initMode bool) (baseline, error) {
	base := baseline{Limit: defaultLimit, Funcs: map[string]float64{}}
	if path == "" {
		return base, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && initMode {
		return base, nil
	}
	if err != nil {
		return base, fmt.Errorf("read baseline: %w", err)
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return base, fmt.Errorf("read baseline: %w", err)
	}
	if base.Funcs == nil {
		base.Funcs = map[string]float64{}
	}
	return base, nil
}

func crap(complexity int, coverage float64) float64 {
	c := float64(complexity)
	return c*c*math.Pow(1-coverage, 3) + c
}

// evaluate sets Allowed on every score and returns the ones above it.
func evaluate(scores []fnScore, ceilings map[string]float64, limit float64) []fnScore {
	var out []fnScore
	for i := range scores {
		allowed := limit
		if c, ok := ceilings[scores[i].Key]; ok && c > allowed {
			allowed = c
		}
		scores[i].Allowed = allowed
		if scores[i].CRAP > allowed+1e-9 {
			out = append(out, scores[i])
		}
	}
	return out
}

// ceilingFor pins a score with 1% headroom (rounded up to a whole number) so
// small run-to-run coverage differences in concurrent code don't flake CI.
func ceilingFor(score float64) float64 { return math.Ceil(score * 1.01) }

func initCeilings(scores []fnScore, limit float64) map[string]float64 {
	out := map[string]float64{}
	for _, s := range scores {
		if s.CRAP > limit {
			out[s.Key] = ceilingFor(s.CRAP)
		}
	}
	return out
}

// ratchetCeilings only lowers existing ceilings; entries for functions that
// are gone or now within the limit are dropped. New offenders are never added.
func ratchetCeilings(scores []fnScore, prev map[string]float64, limit float64) map[string]float64 {
	current := map[string]float64{}
	for _, s := range scores {
		current[s.Key] = s.CRAP
	}
	out := map[string]float64{}
	for key, ceiling := range prev {
		score, ok := current[key]
		if !ok || score <= limit {
			continue
		}
		out[key] = math.Min(ceiling, ceilingFor(score))
	}
	return out
}

func writeBaseline(path string, limit float64, funcs map[string]float64) error {
	if path == "" {
		return fmt.Errorf("-baseline is required with -init/-update")
	}
	b := baseline{
		Comment: "Per-function CRAP ceilings for legacy functions above `limit` (tools/crap). Ceilings may only go down: `make quality-baseline` lowers them after tests improve. Every function not listed must stay at or below `limit`.",
		Limit:   limit,
		Funcs:   funcs,
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func report(w io.Writer, scores, violations []fnScore, limit float64, pinned, top int) {
	sorted := append([]fnScore(nil), scores...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CRAP > sorted[j].CRAP })
	fmt.Fprintf(w, "CRAP gate: %d functions, limit %.0f, %d legacy ceilings\n\n", len(scores), limit, pinned)
	fmt.Fprintf(w, "%-70s %5s %7s %9s %9s\n", "function", "cc", "cover", "crap", "allowed")
	for i, s := range sorted {
		if i >= top {
			break
		}
		fmt.Fprintf(w, "%-70s %5d %6.1f%% %9.2f %9.2f\n", trim(s.Key, 70), s.Complexity, s.Coverage*100, s.CRAP, s.Allowed)
	}
	if len(violations) == 0 {
		fmt.Fprintln(w, "\nCRAP gate passed.")
		return
	}
	fmt.Fprintf(w, "\nCRAP gate failed: %d function(s) above their allowed score.\n", len(violations))
	fmt.Fprintln(w, "Add tests or split the function. If a legacy function legitimately grew, raise its ceiling in quality/crap-baseline.json in the same PR.")
	for _, v := range violations {
		fmt.Fprintf(w, "  - %s: CRAP %.2f > allowed %.2f (complexity %d, coverage %.1f%%)\n", v.Key, v.CRAP, v.Allowed, v.Complexity, v.Coverage*100)
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}

func modulePath(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module line in %s", goMod)
}

// readProfile returns coverage blocks keyed by module-relative file path.
func readProfile(path, module string) (map[string][]block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type key struct {
		file string
		pos  string
	}
	merged := map[key]block{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		file, b, pos, ok := parseProfileLine(line)
		if !ok {
			continue
		}
		file = strings.TrimPrefix(file, module+"/")
		k := key{file, pos}
		if prev, ok := merged[k]; ok && prev.count > b.count {
			b.count = prev.count
		}
		merged[k] = b
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := map[string][]block{}
	for k, b := range merged {
		out[k.file] = append(out[k.file], b)
	}
	return out, nil
}

// parseProfileLine parses "path/file.go:12.3,15.4 2 1".
func parseProfileLine(line string) (string, block, string, bool) {
	colon := strings.LastIndex(line, ":")
	if colon < 0 {
		return "", block{}, "", false
	}
	file, rest := line[:colon], line[colon+1:]
	fields := strings.Fields(rest)
	if len(fields) != 3 {
		return "", block{}, "", false
	}
	start, end, ok := strings.Cut(fields[0], ",")
	if !ok {
		return "", block{}, "", false
	}
	sl, err1 := strconv.Atoi(strings.SplitN(start, ".", 2)[0])
	el, err2 := strconv.Atoi(strings.SplitN(end, ".", 2)[0])
	stmts, err3 := strconv.Atoi(fields[1])
	count, err4 := strconv.Atoi(fields[2])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return "", block{}, "", false
	}
	return file, block{startLine: sl, endLine: el, stmts: stmts, count: count}, fields[0], true
}

func skipDir(rel string, name string) bool {
	if rel == "." {
		return false
	}
	switch name {
	case ".git", "vendor", "node_modules", "testdata", "ui", "target", "examples":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func scoreFunctions(root string, blocks map[string][]block) ([]fnScore, error) {
	var scores []fnScore
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasSuffix(rel, ".pb.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			start := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			cov := functionCoverage(blocks[rel], start, end)
			cc := complexity(fn)
			scores = append(scores, fnScore{
				Key:        rel + ":" + funcName(fn),
				Complexity: cc,
				Coverage:   cov,
				CRAP:       crap(cc, cov),
			})
		}
		return nil
	})
	return scores, err
}

// functionCoverage is the share of statements in [start, end] that were hit.
// Functions without any profiled statements (for example a package that has
// no tests and was not in the profile) count as uncovered.
func functionCoverage(blocks []block, start, end int) float64 {
	total, hit := 0, 0
	for _, b := range blocks {
		if b.startLine < start || b.endLine > end {
			continue
		}
		total += b.stmts
		if b.count > 0 {
			hit += b.stmts
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total)
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + recvType(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func recvType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return "*" + recvType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvType(t.X)
	case *ast.IndexListExpr:
		return recvType(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return "?"
}

// complexity is gocyclo-style cyclomatic complexity, including closures
// declared inside the function.
func complexity(fn *ast.FuncDecl) int {
	c := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			if n.List != nil {
				c++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				c++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}
