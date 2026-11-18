package config

import (
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
