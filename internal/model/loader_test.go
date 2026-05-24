package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/parser"
)

func TestLoadModelsFolderCapture(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	
	// Create marketing folder
	marketingDir := filepath.Join(modelsDir, "marts", "marketing")
	if err := os.MkdirAll(marketingDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create a root model
	rootModelPath := filepath.Join(modelsDir, "root_model.sql")
	if err := os.WriteFile(rootModelPath, []byte("SELECT * FROM events"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a nested model
	nestedModelPath := filepath.Join(marketingDir, "marketing_model.sql")
	if err := os.WriteFile(nestedModelPath, []byte("SELECT * FROM users"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create a nested model with explicit schema override
	explicitModelPath := filepath.Join(marketingDir, "explicit_model.sql")
	explicitSQL := "-- @schema: custom_schema\nSELECT * FROM events"
	if err := os.WriteFile(explicitModelPath, []byte(explicitSQL), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := parser.NewParser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	assets, err := LoadModels(modelsDir, p)
	if err != nil {
		t.Fatal(err)
	}

	if len(assets) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(assets))
	}

	for _, a := range assets {
		switch a.Name {
		case "root_model":
			if _, ok := a.Config["schema"]; ok {
				t.Errorf("Root model should not have an injected schema, got %s", a.Config["schema"])
			}
		case "marketing_model":
			if a.Config["schema"] != "marts_marketing" {
				t.Errorf("Nested model should have 'marts_marketing' schema, got %s", a.Config["schema"])
			}
		case "explicit_model":
			if a.Config["schema"] != "custom_schema" {
				t.Errorf("Explicit model should retain 'custom_schema', got %s", a.Config["schema"])
			}
		}
	}
}
