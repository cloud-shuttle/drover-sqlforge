package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloud-shuttle/drover-sqlforge/internal/config"
)

func TestGatewayRequiresVirtualKeyFromEnv(t *testing.T) {
	cfg := config.AIConfig{Provider: "gateway", Endpoint: "https://gateway.example/v1", Model: "m"}
	if _, err := ClientFromConfig(cfg, func(string) string { return "" }, nil); err == nil {
		t.Fatal("expected missing key error")
	}
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("x-bf-vk") != "vk-test" {
			t.Errorf("vk = %q", r.Header.Get("x-bf-vk"))
		}
		if r.Header.Get("X-Source") != "drover-sqlforge" {
			t.Errorf("source = %q", r.Header.Get("X-Source"))
		}
		if r.Header.Get("x-bf-dim-agent-job-id") != "job-1" {
			t.Errorf("dim = %q", r.Header.Get("x-bf-dim-agent-job-id"))
		}
		if r.Header.Get("x-bf-dim-sql") != "" {
			t.Errorf("unsafe dim was forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"a select"}}]}`))
	}))
	defer srv.Close()
	client, err := ClientFromConfig(cfg, func(key string) string {
		if key != "SQLFORGE_AI_VIRTUAL_KEY" {
			return ""
		}
		return "vk-test"
	}, []string{"X_BF_DIM_AGENT_JOB_ID=job-1", "X_BF_DIM_SQL=SELECT SECRET"})
	if err != nil {
		t.Fatal(err)
	}
	client.Endpoint = srv.URL + "/v1"
	client.HTTP = srv.Client()
	got, err := client.Explain("SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a select" || !called {
		t.Fatalf("got %q called %v", got, called)
	}
	if strings.Contains(errString(client), "vk-test") {
		t.Fatal("client string leaked the key")
	}
}

func errString(c *Client) string {
	return c.Endpoint + c.Model + c.Provider
}

func TestGatewayStatusAndEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	client, err := NewGatewayClient(srv.URL, "m", "vk-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	client.HTTP = srv.Client()
	if _, err := client.Explain("SELECT 1"); err == nil || strings.Contains(err.Error(), "vk-test") {
		t.Fatal(err)
	}

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer empty.Close()
	client.Endpoint = empty.URL
	client.HTTP = empty.Client()
	if _, err := client.Explain("SELECT 1"); err == nil {
		t.Fatal("expected empty choices error")
	}
}

func TestGatewayRefusesMissingKeyBeforeDial(t *testing.T) {
	client := &Client{Provider: "gateway", Endpoint: "http://127.0.0.1:1"}
	if _, err := client.Explain("SELECT 1"); err == nil || !strings.Contains(err.Error(), "SQLFORGE_AI_VIRTUAL_KEY") {
		t.Fatal(err)
	}
}
