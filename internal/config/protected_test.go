package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func loadYAML(t *testing.T, body string) *Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sqlforge.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestProtectedEnvironments(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want []string
	}{
		{"default when unset", "name: p\n", []string{"prod"}},
		{"explicit list", "name: p\nprotected_environments: [prod, staging]\n", []string{"prod", "staging"}},
		{"explicit empty protects nothing", "name: p\nprotected_environments: []\n", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := loadYAML(t, c.yaml).Protected()
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Protected() = %v, want %v", got, c.want)
			}
		})
	}
	var nilCfg *Config
	if got := nilCfg.Protected(); !reflect.DeepEqual(got, []string{"prod"}) {
		t.Fatalf("nil config Protected() = %v", got)
	}
}
