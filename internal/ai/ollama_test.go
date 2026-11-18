package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExplainOllamaSuccessAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`{"response":"selects one row"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "llama")
	got, err := client.Explain("SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "selects one row" {
		t.Fatalf("got %q", got)
	}

	status := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer status.Close()
	client.Endpoint = status.URL
	if _, err := client.Explain("SELECT 1"); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("status error = %v", err)
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	defer badJSON.Close()
	client.Endpoint = badJSON.URL
	if _, err := client.Explain("SELECT 1"); err == nil {
		t.Fatal("expected decode error")
	}

	client.Endpoint = "http://127.0.0.1:1"
	if _, err := client.Explain("SELECT 1"); err == nil {
		t.Fatal("expected dial error")
	}

	client.Endpoint = "http://\n"
	if _, err := client.Explain("SELECT 1"); err == nil {
		t.Fatal("expected request error")
	}
}
