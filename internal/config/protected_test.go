package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProtectedSet(t *testing.T) {
	if got := (*Config)(nil).ProtectedSet(); got != nil {
		t.Fatalf("nil config = %#v, want nil (default prod)", got)
	}

	var omitted Config
	if err := yaml.Unmarshal([]byte("name: demo\n"), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.ProtectedEnvironments != nil || omitted.ProtectedSet() != nil {
		t.Fatalf("omitted field = %#v", omitted.ProtectedEnvironments)
	}

	var empty Config
	if err := yaml.Unmarshal([]byte("protected_environments: []\n"), &empty); err != nil {
		t.Fatal(err)
	}
	set := empty.ProtectedSet()
	if set == nil || len(set) != 0 {
		t.Fatalf("explicit empty = %#v, want empty non-nil map", set)
	}

	var listed Config
	if err := yaml.Unmarshal([]byte("protected_environments: [prod, staging]\n"), &listed); err != nil {
		t.Fatal(err)
	}
	set = listed.ProtectedSet()
	if _, ok := set["prod"]; !ok {
		t.Fatal("missing prod")
	}
	if _, ok := set["staging"]; !ok {
		t.Fatal("missing staging")
	}
}

func TestAIConfigIgnoresVirtualKey(t *testing.T) {
	var cfg Config
	raw := "ai:\n  provider: gateway\n  endpoint: https://gateway.example/v1\n  virtual_key: super-secret\n"
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AI.Provider != "gateway" {
		t.Fatalf("provider = %q", cfg.AI.Provider)
	}
	encoded, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret") || strings.Contains(string(encoded), "virtual_key") {
		t.Fatalf("virtual key was stored in config: %s", encoded)
	}
}
