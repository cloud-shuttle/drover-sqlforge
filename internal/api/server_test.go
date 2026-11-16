package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/graph"
	"github.com/cloud-shuttle/drover-sqlforge/internal/model"
)

// fakeRunner records every query it is asked to run.
type fakeRunner struct {
	queries []string
}

func (f *fakeRunner) Exec(ctx context.Context, sql string) error { return nil }
func (f *fakeRunner) CreateSchemaDDL(schema string) string       { return "" }
func (f *fakeRunner) CreateTableDDL(schema, table, selectSQL string) string {
	return ""
}
func (f *fakeRunner) CreateViewDDL(schema, table, selectSQL string) string { return "" }
func (f *fakeRunner) CreateMaterializedViewDDL(schema, table, selectSQL string) string {
	return ""
}
func (f *fakeRunner) CreateStreamingTableDDL(schema, table string, config map[string]string) string {
	return ""
}
func (f *fakeRunner) TableExists(ctx context.Context, schema, table string) (bool, error) {
	return true, nil
}
func (f *fakeRunner) CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string {
	return ""
}
func (f *fakeRunner) QueryCount(ctx context.Context, sql string) (int, error) { return 0, nil }
func (f *fakeRunner) QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error) {
	f.queries = append(f.queries, sql)
	return []map[string]interface{}{{"id": 1}}, nil
}
func (f *fakeRunner) Name() string { return "fake" }

func testDAG() *graph.DAG {
	d := graph.NewDAG()
	d.AddAsset(&model.Asset{Name: "orders", Config: map[string]string{"materialized": "view"}})
	return d
}

func TestServeModelPreview_KnownModel(t *testing.T) {
	r := &fakeRunner{}
	h := ServeModelPreview(r, "sqlforge__dev", testDAG())
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/models/orders/preview", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if len(r.queries) != 1 || r.queries[0] != "SELECT * FROM sqlforge__dev.orders LIMIT 50" {
		t.Fatalf("queries = %q", r.queries)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected CORS header %q", got)
	}
}

func TestServeModelPreview_RejectsInjection(t *testing.T) {
	paths := []string{
		"/api/models/orders;DROP%20TABLE%20x/preview",
		"/api/models/orders%20UNION%20SELECT%20*%20FROM%20secrets/preview",
		"/api/models/x%3BDELETE%20FROM%20y/preview",
		"/api/models/unknown/preview",
		"/api/models//preview",
		"/api/models/orders/preview/extra",
		"/api/models/orders",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			r := &fakeRunner{}
			h := ServeModelPreview(r, "sqlforge__dev", testDAG())
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code == http.StatusOK {
				t.Fatalf("status = 200, want rejection")
			}
			if len(r.queries) != 0 {
				t.Fatalf("runner was called: %q", r.queries)
			}
		})
	}
}

func TestServeModelPreview_RejectsHostileModelInDAG(t *testing.T) {
	// A model whose name is not a bare identifier (e.g. from an odd file name)
	// must never be interpolated even though it exists in the DAG.
	d := testDAG()
	d.AddAsset(&model.Asset{Name: "bad name;drop"})
	r := &fakeRunner{}
	h := ServeModelPreview(r, "sqlforge__dev", d)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/models/bad%20name;drop/preview", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(r.queries) != 0 {
		t.Fatalf("runner was called: %q", r.queries)
	}
}

func TestServeModelPreview_RejectsHostileSchema(t *testing.T) {
	r := &fakeRunner{}
	h := ServeModelPreview(r, "sqlforge__dev; DROP SCHEMA prod", testDAG())
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/models/orders/preview", nil))
	if rec.Code != http.StatusBadRequest || len(r.queries) != 0 {
		t.Fatalf("status = %d, queries = %q", rec.Code, r.queries)
	}
}

func TestServeDAGAndDetails_NoWildcardCORS(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"dag":     ServeDAG(testDAG()),
		"details": ServeModelDetails(testDAG()),
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/api/models/orders", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", name, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("%s: unexpected CORS header %q", name, got)
		}
	}
}

func TestValidIdentifier(t *testing.T) {
	ok := []string{"orders", "_tmp", "sqlforge__peter_dev", "A1"}
	bad := []string{"", "1abc", "a b", "a;b", "a.b", "a-b", "a'b", `a"b`, "a\nb"}
	for _, s := range ok {
		if !ValidIdentifier(s) {
			t.Errorf("ValidIdentifier(%q) = false", s)
		}
	}
	for _, s := range bad {
		if ValidIdentifier(s) {
			t.Errorf("ValidIdentifier(%q) = true", s)
		}
	}
}

func TestLoopbackHostGuard(t *testing.T) {
	h := LoopbackHostGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cases := map[string]int{
		"localhost:8080":    http.StatusOK,
		"127.0.0.1:8080":    http.StatusOK,
		"[::1]:8080":        http.StatusOK,
		"LOCALHOST":         http.StatusOK,
		"evil.example:8080": http.StatusForbidden,
		"192.168.1.10:8080": http.StatusForbidden,
		"":                  http.StatusForbidden,
	}
	for host, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/dag", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("host %q: status = %d, want %d", host, rec.Code, want)
		}
	}
}
