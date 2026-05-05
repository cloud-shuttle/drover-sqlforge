package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "sqlforge",
	Version: "v0.1.0-alpha",
	Short:   "sqlforge — Modern SQL transformation engine with virtual environments",
	Long: `A fast, Go-native alternative to dbt with Polyglot-powered SQL intelligence,
zero-copy virtual environments, plan/apply workflow, and light AI assistance.`,
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
