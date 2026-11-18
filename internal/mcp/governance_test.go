package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/project"
	"github.com/cloud-shuttle/drover-sqlforge/internal/state"
)

type staticVerifier struct {
	actor Actor
}

func (s staticVerifier) Verify(token string) (Actor, error) {
	if token != "good-token" {
		return Actor{}, &AuthError{Class: "bad_signature"}
	}
	return s.actor, nil
}

func governanceServer(t *testing.T, actor Actor) (*Server, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	server := NewServer("test-api-key", nil)
	server.Audit = NewAuditLog(buf)
	if err := server.UseGovernance(Governance{
		AuthMode: "oidc",
		Verifier: staticVerifier{actor: actor},
	}); err != nil {
		t.Fatal(err)
	}
	server.Registry.Register(Tool{
		Name: "get_model",
		Handler: func(context.Context, []byte) (any, error) {
			return map[string]interface{}{
				"name":      "orders",
				"sql":       "SELECT SECRET_SQL",
				"ast_type":  "select",
				"ast_value": "SECRET_AST",
				"config":    map[string]string{"materialized": "view"},
			}, nil
		},
	})
	return server, buf
}

func postMCP(server *Server, body, token string, extra http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	if token != "" {
		req.Header.Set(defaultOIDCHeader, token)
	}
	for key, values := range extra {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func TestOIDCIgnoresHeaderIdentity(t *testing.T) {
	server, buf := governanceServer(t, Actor{Subject: "user-1", Kind: "member"})
	extra := http.Header{}
	extra.Set("X-User-ID", "injected-user")
	extra.Set("X-Org-ID", "injected-org")
	extra.Set("Authorization", "Bearer test-api-key")
	rec := postMCP(server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "", extra)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if strings.Contains(buf.String(), "injected-user") || strings.Contains(buf.String(), "injected-org") || strings.Contains(buf.String(), "test-api-key") {
		t.Fatalf("audit leaked header identity: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"outcome":"auth_failed"`) || !strings.Contains(buf.String(), `"error_class":"missing_token"`) {
		t.Fatalf("audit = %s", buf.String())
	}
}

func TestGetModelRedactsSQLForAgents(t *testing.T) {
	server, buf := governanceServer(t, Actor{Subject: "agent-1", OrgID: "org-1", Kind: "agent"})
	body := `{"jsonrpc":"2.0","id":1,"method":"get_model","params":{"model_name":"orders","proposed_sql":"SELECT SECRET_SQL","dsn":"postgres://user:pass@db/app"}}`
	rec := postMCP(server, body, "good-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "SECRET_SQL") || strings.Contains(rec.Body.String(), "SECRET_AST") {
		t.Fatalf("agent response leaked SQL: %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "orders") {
		t.Fatalf("agent response dropped the model name: %s", rec.Body)
	}
	line := buf.String()
	for _, secret := range []string{"SECRET_SQL", "SECRET_AST", "postgres://", "good-token", "proposed_sql"} {
		if strings.Contains(line, secret) {
			t.Fatalf("audit contains %q: %s", secret, line)
		}
	}
	if !strings.Contains(line, `"outcome":"redacted"`) || !strings.Contains(line, `"sub":"agent-1"`) || !strings.Contains(line, `"models":["orders"]`) {
		t.Fatalf("audit = %s", line)
	}
}

func TestGetModelKeepsSQLForMembers(t *testing.T) {
	server, _ := governanceServer(t, Actor{Subject: "member-1", Kind: "member"})
	body := `{"jsonrpc":"2.0","id":1,"method":"get_model","params":{"model_name":"orders"}}`
	rec := postMCP(server, body, "good-token", nil)
	if !strings.Contains(rec.Body.String(), "SECRET_SQL") {
		t.Fatalf("member response = %s", rec.Body)
	}
}

func TestHookDenySkipsTheTool(t *testing.T) {
	server, buf := governanceServer(t, Actor{Subject: "agent-1", Kind: "agent"})
	called := false
	server.Registry.Register(Tool{
		Name: "apply_change",
		Handler: func(context.Context, []byte) (any, error) {
			called = true
			return nil, nil
		},
	})
	server.Hooks.hooks = append([]ToolHook{denyHook{}}, server.Hooks.hooks...)
	rec := postMCP(server, `{"jsonrpc":"2.0","id":1,"method":"apply_change","params":{"plan_id":"pln_1","proposed_sql":"SELECT SECRET_SQL"}}`, "good-token", nil)
	if called {
		t.Fatal("denied tool ran")
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != InternalError {
		t.Fatalf("error = %+v", resp.Error)
	}
	if strings.Contains(buf.String(), "SECRET_SQL") || !strings.Contains(buf.String(), `"outcome":"deny"`) {
		t.Fatalf("audit = %s", buf.String())
	}
}

func TestHookRequiresApproval(t *testing.T) {
	server, _ := governanceServer(t, Actor{Subject: "agent-1", Kind: "agent"})
	server.Hooks.hooks = []ToolHook{approvalHook{}}
	rec := postMCP(server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "good-token", nil)
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != ApprovalRequired {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestHookPanicFailsClosed(t *testing.T) {
	chain := &HookChain{hooks: []ToolHook{panicHook{}}}
	verdict := chain.PreTool(context.Background(), &ToolCall{Tool: "apply_change"})
	if verdict.Action != "deny" || verdict.PolicyID != "hook_panic" {
		t.Fatalf("verdict = %+v", verdict)
	}
}

func TestAuditDropsToolErrorSQL(t *testing.T) {
	server, buf := governanceServer(t, Actor{Subject: "member-1", Kind: "member"})
	server.Registry.Register(Tool{
		Name: "plan_change",
		Handler: func(context.Context, []byte) (any, error) {
			return nil, errString("warehouse failed SELECT SECRET_SQL postgres://user:pass@db key=SUPERSECRET")
		},
	})
	body := `{"jsonrpc":"2.0","id":1,"method":"plan_change","params":{"model_name":"orders;drop","plan_id":"not a plan","proposed_sql":"SELECT SECRET_SQL"}}`
	rec := postMCP(server, body, "good-token", nil)
	if !strings.Contains(rec.Body.String(), "SECRET_SQL") {
		t.Fatal("caller-visible error should still explain the failure")
	}
	line := buf.String()
	for _, secret := range []string{"SECRET_SQL", "postgres://", "SUPERSECRET", "orders;drop", "not a plan"} {
		if strings.Contains(line, secret) {
			t.Fatalf("audit contains %q: %s", secret, line)
		}
	}
	if !strings.Contains(line, `"error_class":"error"`) {
		t.Fatalf("audit = %s", line)
	}
}

func TestAPIKeyActorAndLocalActor(t *testing.T) {
	buf := &bytes.Buffer{}
	server := NewServer("test-api-key", nil)
	server.Audit = NewAuditLog(buf)
	rec := postMCP(server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "", http.Header{"Authorization": {"Bearer test-api-key"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("api key status %d", rec.Code)
	}
	if !strings.Contains(buf.String(), `"sub":"api-key"`) || !strings.Contains(buf.String(), `"via":"api_key"`) {
		t.Fatalf("audit = %s", buf.String())
	}

	buf.Reset()
	open := NewServer("", nil)
	open.Audit = NewAuditLog(buf)
	rec = postMCP(open, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("local status %d", rec.Code)
	}
	if !strings.Contains(buf.String(), `"sub":"local"`) || !strings.Contains(buf.String(), `"via":"local"`) {
		t.Fatalf("audit = %s", buf.String())
	}
}

func TestWardenHookIsRejectedWithoutTheTag(t *testing.T) {
	server := NewServer("", nil)
	err := server.UseGovernance(Governance{HookNames: []string{"warden"}})
	if err == nil || !strings.Contains(err.Error(), "drover-warden") {
		t.Fatal(err)
	}
}

func TestPreToolCanRewriteArgs(t *testing.T) {
	server, _ := governanceServer(t, Actor{Subject: "member-1", Kind: "member"})
	var got []byte
	server.Registry.Register(Tool{
		Name: "plan_change",
		Handler: func(_ context.Context, params []byte) (any, error) {
			got = append([]byte(nil), params...)
			return map[string]any{"plan_id": "pln_ok"}, nil
		},
	})
	server.Hooks.hooks = []ToolHook{rewriteHook{}, redactHook{}}
	rec := postMCP(server, `{"jsonrpc":"2.0","id":1,"method":"plan_change","params":{"model_name":"orders","proposed_sql":"SELECT SECRET_SQL"}}`, "good-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(string(got), "SECRET_SQL") || !strings.Contains(string(got), "SELECT 1") {
		t.Fatalf("handler params = %s", got)
	}
}

type rewriteHook struct{}

func (rewriteHook) PreTool(context.Context, *ToolCall) Verdict {
	return Verdict{Action: "redact", Modified: map[string]any{"model_name": "orders", "proposed_sql": "SELECT 1"}}
}
func (rewriteHook) PostTool(context.Context, *ToolCall, any) Verdict { return allowVerdict() }

func TestUnknownHookAndHostileEnvironment(t *testing.T) {
	server := NewServer("", nil)
	if err := server.UseGovernance(Governance{HookNames: []string{"nope"}}); err == nil {
		t.Fatal("expected unknown hook error")
	}
	server.Runtime = &project.Runtime{Env: &state.Environment{Name: "prod;drop"}}
	if server.sessionEnvironment() != "" {
		t.Fatal("hostile environment was accepted")
	}
	server.Runtime.Env.Name = "dev"
	if server.sessionEnvironment() != "dev" {
		t.Fatal("dev should be a valid environment name")
	}
}

func TestWardenRequestMapping(t *testing.T) {
	got := WardenRequestFrom(&ToolCall{
		Tool:       "apply_change",
		PolicyTool: "sqlforge_apply_change",
		Args:       map[string]any{"plan_id": "pln_1"},
		Actor:      Actor{OrgID: "org-9"},
	})
	if got.ToolName != "sqlforge_apply_change" || got.TenantID != "org-9" || got.Args["plan_id"] != "pln_1" {
		t.Fatalf("request = %+v", got)
	}
	if WardenRequestFrom(nil).ToolName != "" {
		t.Fatal("nil call should map to an empty request")
	}
}

type denyHook struct{}

func (denyHook) PreTool(context.Context, *ToolCall) Verdict {
	return Verdict{Action: "deny", Reason: "blocked", PolicyID: "pol-test"}
}
func (denyHook) PostTool(context.Context, *ToolCall, any) Verdict { return allowVerdict() }

type approvalHook struct{}

func (approvalHook) PreTool(context.Context, *ToolCall) Verdict {
	return Verdict{Action: "require_approval", Reason: "needs a person", PolicyID: "pol-hitl"}
}
func (approvalHook) PostTool(context.Context, *ToolCall, any) Verdict { return allowVerdict() }

type panicHook struct{}

func (panicHook) PreTool(context.Context, *ToolCall) Verdict { panic("hook exploded") }
func (panicHook) PostTool(context.Context, *ToolCall, any) Verdict {
	return allowVerdict()
}
