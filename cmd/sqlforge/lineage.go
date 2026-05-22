package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/drover-org/drover-sqlforge/internal/parser"
	"github.com/spf13/cobra"
)

var lineageJSON bool

var lineageCmd = &cobra.Command{
	Use:   "lineage [model_name]",
	Short: "Show column-level lineage for models",
	Long: `Extract output column → upstream column mappings from model SQL.

Uses structural SELECT/FROM parsing (v1). See docs/explanation/GAP_ANALYSIS.md.`,
	Run: runLineage,
}

func init() {
	lineageCmd.Flags().BoolVar(&lineageJSON, "json", false, "Emit JSON")
	rootCmd.AddCommand(lineageCmd)
}

func runLineage(cmd *cobra.Command, args []string) {
	rt, err := loadRuntime("prod")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer rt.Close()

	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}

	type modelLineage struct {
		Model   string                 `json:"model"`
		Columns []parser.ColumnMapping `json:"columns"`
	}

	var report []modelLineage
	for name, node := range rt.DAG.Nodes {
		if filter != "" && name != filter {
			continue
		}
		cols, err := rt.Parser.ExtractColumnLineage(node.SQL)
		if err != nil {
			fmt.Printf("Error: lineage for %s: %v\n", name, err)
			os.Exit(1)
		}
		report = append(report, modelLineage{Model: name, Columns: cols})
	}

	if filter != "" && len(report) == 0 {
		fmt.Printf("Error: model %q not found\n", filter)
		os.Exit(1)
	}

	if lineageJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return
	}

	for _, ml := range report {
		fmt.Printf("\n%s\n", ml.Model)
		for _, col := range ml.Columns {
			if len(col.Sources) == 0 {
				fmt.Printf("  %s (no column refs)\n", col.Output)
				continue
			}
			for _, src := range col.Sources {
				fmt.Printf("  %s <- %s.%s\n", col.Output, src.Relation, src.Column)
			}
		}
	}
	if len(report) == 0 {
		fmt.Println("No models found.")
	}
}
