package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/model"
)

type recordingRunner struct{ queries []string }

func (f *recordingRunner) Exec(ctx context.Context, sql string) error { return nil }
func (f *recordingRunner) CreateSchemaDDL(string) string              { return "" }
func (f *recordingRunner) CreateTableDDL(_, _, _ string) string       { return "" }
func (f *recordingRunner) CreateViewDDL(_, _, _ string) string        { return "" }
func (f *recordingRunner) CreateMaterializedViewDDL(_, _, _ string) string {
	return ""
}
func (f *recordingRunner) CreateStreamingTableDDL(_, _ string, _ map[string]string) string {
	return ""
}
func (f *recordingRunner) TableExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *recordingRunner) CreateIncrementalMergeDDL(_, _, _ string, _ map[string]string) string {
	return ""
}
func (f *recordingRunner) QueryCount(context.Context, string) (int, error) { return 0, nil }
func (f *recordingRunner) QueryData(_ context.Context, sql string) ([]map[string]interface{}, error) {
	f.queries = append(f.queries, sql)
	return nil, nil
}
func (f *recordingRunner) Name() string { return "recording" }

func uiTestHandler(bindHost string) (http.Handler, *recordingRunner) {
	d := graph.NewDAG()
	d.AddAsset(&model.Asset{Name: "orders"})
	r := &recordingRunner{}
	dist := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	return newUIHandler(d, r, "sqlforge__dev", dist, bindHost), r
}

func TestUIHandler_LoopbackRejectsForeignHost(t *testing.T) {
	h, r := uiTestHandler("127.0.0.1")
	req := httptest.NewRequest(http.MethodGet, "/api/models/orders/preview", nil)
	req.Host = "attacker.example:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(r.queries) != 0 {
		t.Fatalf("runner called: %q", r.queries)
	}
}

func TestUIHandler_RoutesPreviewAndDetails(t *testing.T) {
	h, r := uiTestHandler("127.0.0.1")
	for _, path := range []string{"/api/models/orders/preview", "/api/models/orders", "/api/dag", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "localhost:8080"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", path, rec.Code)
		}
	}
	if len(r.queries) != 1 {
		t.Fatalf("expected exactly one preview query, got %q", r.queries)
	}
}

func TestUIHandler_NonLoopbackBindSkipsHostGuard(t *testing.T) {
	h, _ := uiTestHandler("0.0.0.0")
	req := httptest.NewRequest(http.MethodGet, "/api/dag", nil)
	req.Host = "sqlforge.internal:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
