package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/drover-org/drover-sqlforge/internal/plan"
)

var exportOutputFlag string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export metadata or DAG artifacts",
}

var exportDagCmd = &cobra.Command{
	Use:   "dag [environment]",
	Short: "Export the full DAG and execution targets as JSON for external orchestrators",
	Run: func(cmd *cobra.Command, args []string) {
		envName := "dev"
		if len(args) > 0 {
			envName = args[0]
		}

		rt, err := loadRuntime(envName)
		if err != nil {
			fmt.Printf("Error loading runtime: %v\n", err)
			os.Exit(1)
		}
		defer rt.Close()

		exportDAG := plan.GenerateDAGExport(envName, rt.Env.Schema, rt.Assets, rt.DAG)

		data, err := json.MarshalIndent(exportDAG, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}

		if exportOutputFlag != "" {
			if err := os.WriteFile(exportOutputFlag, data, 0644); err != nil {
				fmt.Printf("Error writing file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully exported DAG to %s\n", exportOutputFlag)
		} else {
			fmt.Println(string(data))
		}
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.AddCommand(exportDagCmd)

	exportDagCmd.Flags().StringVarP(&exportOutputFlag, "output", "o", "", "Output JSON file path")
}
