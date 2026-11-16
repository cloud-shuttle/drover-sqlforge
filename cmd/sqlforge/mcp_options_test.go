package main

import (
	"reflect"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/config"
)

func TestMCPOptions(t *testing.T) {
	list := []string{"prod", "finance"}
	opts := mcpOptions(&config.Config{ProtectedEnvironments: &list}, true, false)
	if !opts.ReadOnly || opts.AllowProtectedApply {
		t.Fatalf("flags not carried: %+v", opts)
	}
	if !reflect.DeepEqual(opts.ProtectedEnvironments, list) {
		t.Fatalf("protected = %v", opts.ProtectedEnvironments)
	}
	def := mcpOptions(&config.Config{}, false, true)
	if !reflect.DeepEqual(def.ProtectedEnvironments, []string{"prod"}) || !def.AllowProtectedApply {
		t.Fatalf("defaults = %+v", def)
	}
	for _, name := range []string{"read-only", "allow-protected-apply"} {
		if mcpCmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
}
