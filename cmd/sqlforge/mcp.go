package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/drover-org/drover-sqlforge/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpPort   string
	mcpLocal  bool
	mcpAPIKey string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the Model Context Protocol (MCP) server for autonomous agents",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Initializing MCP Server...\n")

		envName := "dev"
		if len(args) > 0 {
			envName = args[0]
		}

		fmt.Printf("Loading context for environment: %s\n", envName)
		_, dag, _, _, semanticLayer, err := runPipeline(envName)
		if err != nil {
			fmt.Printf("Failed to load project context: %v\n", err)
			os.Exit(1)
		}

		// Create MCP server instance
		server := mcp.NewServer(mcpAPIKey, dag, semanticLayer)

		// Register the JSON-RPC route
		http.Handle("/mcp", server)

		// Binding address
		addr := "0.0.0.0"
		if mcpLocal {
			addr = "127.0.0.1"
		}
		bind := fmt.Sprintf("%s:%s", addr, mcpPort)

		fmt.Printf("🚀 SQLForge MCP Server running at http://%s/mcp\n", bind)
		if mcpAPIKey != "" {
			fmt.Printf("🔒 Authentication: Enabled (API Key required)\n")
		} else {
			fmt.Printf("⚠️  Authentication: Disabled (Local Mode)\n")
		}

		err = http.ListenAndServe(bind, nil)
		if err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpPort, "port", "8080", "Port to bind the MCP server to")
	mcpCmd.Flags().BoolVar(&mcpLocal, "local", true, "Bind to 127.0.0.1 instead of 0.0.0.0")
	mcpCmd.Flags().StringVar(&mcpAPIKey, "api-key", "", "API Key required for access")

	rootCmd.AddCommand(mcpCmd)
}
