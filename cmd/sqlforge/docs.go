package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/plan"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Manage and generate data documentation",
}

var docsGenerateCmd = &cobra.Command{
	Use:   "generate [environment]",
	Short: "Generate static data catalog documentation",
	Run: func(cmd *cobra.Command, args []string) {
		envName := "dev"
		if len(args) > 0 {
			envName = args[0]
		}

		fmt.Printf("Loading DAG metadata for environment: %s\n", envName)
		rt, err := loadRuntime(envName)
		if err != nil {
			fmt.Printf("Failed to load project: %v\n", err)
			os.Exit(1)
		}
		defer rt.Close()

		fmt.Println("Compiling structural column lineage and configurations...")
		catalog := plan.GenerateCatalogExport(envName, rt.Env.Schema, rt.Assets, rt.DAG, rt.Semantic, rt.Parser)

		// Serialize to JSON
		catalogJSON, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			fmt.Printf("Failed to serialize catalog: %v\n", err)
			os.Exit(1)
		}

		// Inject into embedded HTML template
		startMarker := "/* SQLFORGE_CATALOG_INJECT_START */"
		endMarker := "/* SQLFORGE_CATALOG_INJECT_END */"
		startIndex := strings.Index(plan.DocsTemplate, startMarker)
		endIndex := strings.Index(plan.DocsTemplate, endMarker)

		var outputHTML string
		if startIndex != -1 && endIndex != -1 && startIndex < endIndex {
			outputHTML = plan.DocsTemplate[:startIndex] + "\n\t\twindow.SQLFORGE_CATALOG = " + string(catalogJSON) + ";\n\t\t" + plan.DocsTemplate[endIndex+len(endMarker):]
		} else {
			// Fallback to simple replace
			injectMarker := "/* SQLFORGE_CATALOG_INJECT */"
			replacement := fmt.Sprintf("window.SQLFORGE_CATALOG = %s;", string(catalogJSON))
			if !strings.Contains(plan.DocsTemplate, injectMarker) {
				fmt.Printf("Error: embedded HTML template is missing the catalog injection marker.\n")
				os.Exit(1)
			}
			outputHTML = strings.Replace(plan.DocsTemplate, injectMarker, replacement, 1)
		}

		// Ensure target directory exists
		targetDir := "target"
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			fmt.Printf("Failed to create target directory: %v\n", err)
			os.Exit(1)
		}

		outputPath := filepath.Join(targetDir, "index.html")
		if err := os.WriteFile(outputPath, []byte(outputHTML), 0644); err != nil {
			fmt.Printf("Failed to write static documentation file: %v\n", err)
			os.Exit(1)
		}

		absPath, _ := filepath.Abs(outputPath)
		fmt.Printf("\n✨ Static Data Catalog generated successfully!\n")
		fmt.Printf("📂 Location: %s\n\n", absPath)
		fmt.Printf("👉 Double-click the file to open your interactive lineage DAG locally in any browser (Zero-CORS required)!\n")
	},
}

func init() {
	docsCmd.AddCommand(docsGenerateCmd)
}
