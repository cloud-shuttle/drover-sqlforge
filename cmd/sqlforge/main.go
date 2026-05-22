package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "sqlforge",
	Version: "v0.1.0-alpha",
	Short:   "sqlforge — SQL transformation engine with environments and plan/apply",
	Long: `A fast, Go-native alternative to dbt with Polyglot-powered SQL intelligence,
isolated environments, zero-copy isolation on supported warehouses, plan/apply workflow, and semantic metrics.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
