package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/plan"
	"github.com/cloud-shuttle/drover-sqlforge/internal/project"
	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
)

func toolNames(reg *Registry) map[string]bool {
	names := map[string]bool{}
	for _, tool := range reg.ListTools() {
		names[tool.Name] = true
	}
	return names
}

func TestUseReadOnlyOmitsWriteTools(t *testing.T) {
	server := NewServer("", nil)
	server.UseReadOnly()

	names := toolNames(server.Registry)
	for _, read := range []string{"list_models", "get_model", "list_metrics", "query_metric", "list_snapshots"} {
		if !names[read] {
			t.Errorf("read-only registry missing %s", read)
		}
	}
	for _, write := range writeToolNames {
		if names[write] {
			t.Errorf("read-only registry still has %s", write)
		}
		if _, ok := server.Registry.Get(write); ok {
			t.Errorf("Get(%s) still succeeds", write)
		}
	}
}

func TestServerReadOnlyToolsList(t *testing.T) {
	server := NewServer("test-api-key", nil)
	server.UseReadOnly()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-api-key")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var resp struct {
		Result []struct {
			Name string `json:"name"`
		} `json:"result"`
		Error *Error `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
	got := map[string]bool{}
	for _, tool := range resp.Result {
		got[tool.Name] = true
	}
	for _, write := range writeToolNames {
		if got[write] {
			t.Errorf("tools/list included %s", write)
		}
	}
	if !got["get_model"] || !got["list_snapshots"] {
		t.Fatalf("tools/list = %#v", got)
	}

	applyBody := `{"jsonrpc":"2.0","id":2,"method":"apply_change","params":{"plan_id":"x"}}`
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(applyBody))
	req.Header.Set("Authorization", "Bearer test-api-key")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)
	var denied Response
	if err := json.Unmarshal(w.Body.Bytes(), &denied); err != nil {
		t.Fatal(err)
	}
	if denied.Error == nil || denied.Error.Code != MethodNotFound {
		t.Fatalf("apply_change in read-only = %+v", denied.Error)
	}
}

func noopPlan(env string) *plan.ExecutionPlan {
	p := &plan.ExecutionPlan{}
	if env != "" {
		p.Environment = &state.Environment{Name: env}
	}
	return p
}

func TestApplyChangeProtectedAndBoundEnvironment(t *testing.T) {
	store := NewPlanStore()
	id, err := store.PutProposal(noopPlan("prod"), "orders", "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	pending, ok := store.GetPending(id)
	if !ok || pending.Environment != "prod" {
		t.Fatalf("stored environment = %q ok=%v", pending.Environment, ok)
	}

	prod := &project.Runtime{Env: &state.Environment{Name: "prod"}}
	_, err = applyChange(context.Background(), prod, store, id)
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("default policy err = %v", err)
	}
	if _, ok := store.GetPending(id); !ok {
		t.Fatal("a policy refusal must keep the plan")
	}

	allowed := withApplyPolicy(context.Background(), ApplyPolicy{AllowProtectedApply: true})
	res, err := applyChange(allowed, prod, store, id)
	if err != nil {
		t.Fatal(err)
	}
	if res["status"] != "noop" {
		t.Fatalf("result = %#v", res)
	}

	store = NewPlanStore()
	id, err = store.PutProposal(noopPlan("prod"), "orders", "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	open := withApplyPolicy(context.Background(), ApplyPolicy{Protected: map[string]struct{}{}})
	if _, err := applyChange(open, prod, store, id); err != nil {
		t.Fatalf("explicit empty protected set: %v", err)
	}

	store = NewPlanStore()
	id, err = store.PutProposal(noopPlan("dev"), "orders", "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	dev := &project.Runtime{Env: &state.Environment{Name: "dev"}}
	if _, err := applyChange(context.Background(), dev, store, id); err != nil {
		t.Fatalf("dev is not protected by default: %v", err)
	}

	store = NewPlanStore()
	id, err = store.PutProposal(noopPlan("staging"), "orders", "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	staging := &project.Runtime{Env: &state.Environment{Name: "staging"}}
	custom := withApplyPolicy(context.Background(), ApplyPolicy{Protected: map[string]struct{}{"staging": {}}})
	_, err = applyChange(custom, staging, store, id)
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("custom protected err = %v", err)
	}

	store = NewPlanStore()
	id, err = store.PutProposal(noopPlan("dev"), "orders", "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyChange(allowed, prod, store, id)
	if err == nil || !strings.Contains(err.Error(), "cannot be applied") {
		t.Fatalf("cross-env err = %v", err)
	}
}

func TestApplyPolicyFromBackgroundIsDefaultProd(t *testing.T) {
	policy := applyPolicyFrom(context.Background())
	if !policy.IsProtected("prod") || policy.IsProtected("dev") || policy.IsProtected("") {
		t.Fatalf("default policy = %+v", policy)
	}
	if applyPolicyFrom(nil).IsProtected("prod") != true {
		t.Fatal("nil context should use the default protected set")
	}
}
