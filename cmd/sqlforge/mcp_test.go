package main

import "testing"

func TestResolveMCPAPIKey(t *testing.T) {
	env := map[string]string{mcpAPIKeyEnv: "from-env"}
	getenv := func(k string) string { return env[k] }

	if got := resolveMCPAPIKey("from-flag", getenv); got != "from-flag" {
		t.Errorf("flag should win, got %q", got)
	}
	if got := resolveMCPAPIKey("", getenv); got != "from-env" {
		t.Errorf("env fallback, got %q", got)
	}
	if got := resolveMCPAPIKey("", func(string) string { return "" }); got != "" {
		t.Errorf("no key, got %q", got)
	}
}

func TestMCPBindHost(t *testing.T) {
	if mcpBindHost(true) != "127.0.0.1" || mcpBindHost(false) != "0.0.0.0" {
		t.Fatal("unexpected bind hosts")
	}
}
