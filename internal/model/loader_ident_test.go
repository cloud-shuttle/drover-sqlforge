package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadModels_RejectsUnsafeModelNames(t *testing.T) {
	for _, name := range []string{"my-model.sql", "orders;drop.sql", "1orders.sql", "with space.sql"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1"), 0o644); err != nil {
			t.Fatal(err)
		}
		// The name check runs before parsing, so no parser is needed.
		_, err := LoadModels(dir, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid model name") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
