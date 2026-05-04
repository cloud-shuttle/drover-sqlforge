package semantic_test

import (
	"strings"
	"testing"

	"github.com/drover-org/drover-sqlforge/internal/semantic"
)

func TestCompiler_ValidDimensions(t *testing.T) {
	compiler := semantic.NewCompiler("test_schema")
	metric := &semantic.Metric{
		Name:       "test_metric",
		Expression: "SUM(value)",
		Model:      "test_model",
		Dimensions: []string{"dim1", "dim2"},
	}

	sql, err := compiler.Compile(metric, []string{"dim1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(sql, "dim1") {
		t.Errorf("expected SQL to contain 'dim1', got %s", sql)
	}
	if !strings.Contains(sql, "test_schema.test_model") {
		t.Errorf("expected SQL to contain 'test_schema.test_model', got %s", sql)
	}
	if !strings.Contains(sql, "GROUP BY dim1") {
		t.Errorf("expected SQL to contain 'GROUP BY dim1', got %s", sql)
	}
}

func TestCompiler_InvalidDimension(t *testing.T) {
	compiler := semantic.NewCompiler("test_schema")
	metric := &semantic.Metric{
		Name:       "test_metric",
		Expression: "SUM(value)",
		Model:      "test_model",
		Dimensions: []string{"dim1"},
	}

	_, err := compiler.Compile(metric, []string{"invalid_dim"})
	if err == nil {
		t.Fatalf("expected error for invalid dimension")
	}
}

func TestCompiler_InvalidCharacters(t *testing.T) {
	compiler := semantic.NewCompiler("test_schema")
	metric := &semantic.Metric{
		Name:       "test_metric",
		Expression: "SUM(value)",
		Model:      "test_model",
		Dimensions: []string{"dim1"},
	}

	// This attempts to inject a malicious string that is not in the regex
	_, err := compiler.Compile(metric, []string{"dim1;"})
	if err == nil {
		t.Fatalf("expected error for invalid characters")
	}
}

func FuzzCompiler(f *testing.F) {
	f.Add("dim1")
	f.Add("dim2")
	f.Add("SELECT * FROM users")
	f.Add("dim1;")
	f.Add("123_dim")
	f.Add("")

	metric := &semantic.Metric{
		Name:       "fuzz_metric",
		Expression: "COUNT(1)",
		Model:      "fuzz_model",
		Dimensions: []string{"dim1", "dim2", "123_dim"},
	}
	compiler := semantic.NewCompiler("fuzz_schema")

	f.Fuzz(func(t *testing.T, requestedDim string) {
		sql, err := compiler.Compile(metric, []string{requestedDim})

		if err != nil {
			// Expected error cases:
			// 1. Invalid characters
			// 2. Not supported dimension
			return
		}

		// If it succeeded, it MUST be one of the valid dimensions.
		valid := false
		for _, d := range metric.Dimensions {
			if requestedDim == d {
				valid = true
				break
			}
		}

		if !valid {
			t.Errorf("Compiler successfully returned SQL for an invalid dimension string: %s", requestedDim)
		}

		if !strings.Contains(sql, requestedDim) {
			t.Errorf("Generated SQL did not contain the requested dimension: %s\nSQL: %s", requestedDim, sql)
		}
	})
}
