package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/model"
	"github.com/drover-org/drover-sqlforge/internal/plan"
	"github.com/drover-org/drover-sqlforge/internal/project"
	"github.com/drover-org/drover-sqlforge/internal/state"
)

func envRuntime(name string) *project.Runtime {
	rt := guardTestRuntime()
	rt.Env = &state.Environment{Name: name, Schema: "sqlforge__" + name}
	return rt
}

func listToolNames(t *testing.T, s *Server) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	var resp struct {
		Result []struct {
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode tools/list: %v (%s)", err, rec.Body.String())
	}
	names := make([]string, 0, len(resp.Result))
	for _, r := range resp.Result {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return names
}

func TestReadOnlyMode_ToolsListHasOnlyReadTools(t *testing.T) {
	s := NewServerWithOptions("", envRuntime("dev"), Options{ReadOnly: true})
	got := strings.Join(listToolNames(t, s), ",")
	want := "get_model,list_metrics,list_models,list_snapshots,query_metric"
	if got != want {
		t.Fatalf("read-only tools/list = %s, want %s", got, want)
	}
}

func TestReadOnlyMode_MutatingToolsAreMethodNotFound(t *testing.T) {
	s := NewServerWithOptions("", envRuntime("dev"), Options{ReadOnly: true})
	for _, method := range []string{"plan_change", "apply_change", "run_snapshot"} {
		body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{}}`
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body)))
		var resp Response
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Error == nil || resp.Error.Code != MethodNotFound {
			t.Errorf("%s in read-only mode: got %s, want MethodNotFound", method, rec.Body.String())
		}
	}
}

func TestDefaultMode_ExposesMutatingTools(t *testing.T) {
	s := NewServerWithOptions("", envRuntime("dev"), Options{ProtectedEnvironments: []string{"prod"}})
	got := strings.Join(listToolNames(t, s), ",")
	for _, name := range []string{"plan_change", "apply_change", "run_snapshot"} {
		if !strings.Contains(got, name) {
			t.Errorf("tools/list %s missing %s", got, name)
		}
	}
}

func TestProtectedEnvironment_ApplyRefused(t *testing.T) {
	for _, env := range []string{"prod", "PROD", "Prod"} {
		rt := envRuntime(env)
		s := NewServerWithOptions("", rt, Options{ProtectedEnvironments: []string{"prod"}})
		id, _ := s.Plans.PutProposal(&plan.ExecutionPlan{Environment: rt.Env}, "orders", "SELECT 1 AS id")
		tool, _ := s.Registry.Get("apply_change")
		params, _ := json.Marshal(map[string]string{"plan_id": id})
		_, err := tool.Handler(context.Background(), params)
		if err == nil || !strings.Contains(err.Error(), "protected") {
			t.Errorf("env %s: err = %v, want protected refusal", env, err)
		}
		if _, ok := s.Plans.GetPending(id); !ok {
			t.Errorf("env %s: refused plan should stay available for an allowed operator", env)
		}
	}
}

func TestProtectedEnvironment_AllowFlagPermitsApply(t *testing.T) {
	rt := envRuntime("prod")
	s := NewServerWithOptions("", rt, Options{ProtectedEnvironments: []string{"prod"}, AllowProtectedApply: true})
	// An empty plan short-circuits to "noop" without touching the warehouse.
	id, _ := s.Plans.PutProposal(&plan.ExecutionPlan{Environment: rt.Env}, "orders", "SELECT 1 AS id")
	tool, _ := s.Registry.Get("apply_change")
	params, _ := json.Marshal(map[string]string{"plan_id": id})
	out, err := tool.Handler(context.Background(), params)
	if err != nil {
		t.Fatalf("apply with --allow-protected-apply: %v", err)
	}
	if out.(map[string]interface{})["status"] != "noop" {
		t.Fatalf("out = %v", out)
	}
}

func TestUnprotectedEnvironment_ApplyAllowed(t *testing.T) {
	rt := envRuntime("dev")
	s := NewServerWithOptions("", rt, Options{ProtectedEnvironments: []string{"prod"}})
	id, _ := s.Plans.PutProposal(&plan.ExecutionPlan{Environment: rt.Env}, "orders", "SELECT 1 AS id")
	tool, _ := s.Registry.Get("apply_change")
	params, _ := json.Marshal(map[string]string{"plan_id": id})
	if _, err := tool.Handler(context.Background(), params); err != nil {
		t.Fatalf("apply to dev: %v", err)
	}
}

func TestPlanBoundToEnvironment(t *testing.T) {
	store := NewPlanStore()
	prodPlan := &plan.ExecutionPlan{
		Environment:   &state.Environment{Name: "prod", Schema: "sqlforge__prod"},
		ChangedModels: []*model.Asset{{Name: "orders", SQL: "SELECT 1 AS id"}},
	}
	id, _ := store.PutProposal(prodPlan, "orders", "SELECT 1 AS id")
	if p, _ := store.GetPending(id); p.Environment != "prod" {
		t.Fatalf("plan environment = %q, want prod", p.Environment)
	}

	_, err := applyChange(context.Background(), envRuntime("dev"), store, id)
	if err == nil || !strings.Contains(err.Error(), `created for environment "prod"`) {
		t.Fatalf("cross-environment apply err = %v", err)
	}
	if _, ok := store.GetPending(id); ok {
		t.Fatal("cross-environment plan should be discarded")
	}
}

func TestIsProtectedEnvironment(t *testing.T) {
	cases := []struct {
		env       string
		protected []string
		want      bool
	}{
		{"prod", []string{"prod"}, true},
		{"Prod", []string{"prod"}, true},
		{"prod", []string{" prod "}, true},
		{"dev", []string{"prod"}, false},
		{"prod", nil, false},
		{"", []string{""}, false},
		{"staging", []string{"prod", "staging"}, true},
	}
	for _, c := range cases {
		if got := IsProtectedEnvironment(c.env, c.protected); got != c.want {
			t.Errorf("IsProtectedEnvironment(%q, %v) = %v, want %v", c.env, c.protected, got, c.want)
		}
	}
}
