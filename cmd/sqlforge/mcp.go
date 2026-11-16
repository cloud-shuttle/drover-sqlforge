package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/drover-org/drover-sqlforge/internal/config"
	"github.com/drover-org/drover-sqlforge/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpPort   string
	mcpLocal  bool
	mcpAPIKey string

	mcpReadOnly            bool
	mcpAllowProtectedApply bool
)

// mcpOptions builds the MCP tool policy from flags and sqlforge.yml.
func mcpOptions(cfg *config.Config, readOnly, allowProtected bool) mcp.Options {
	return mcp.Options{
		ReadOnly:              readOnly,
		ProtectedEnvironments: cfg.Protected(),
		AllowProtectedApply:   allowProtected,
	}
}

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
		rt, err := loadRuntime(envName)
		if err != nil {
			fmt.Printf("Failed to load project context: %v\n", err)
			os.Exit(1)
		}
		defer rt.Close()

		cfg, err := config.LoadConfig(".")
		if err != nil {
			fmt.Printf("Failed to load sqlforge.yml: %v\n", err)
			os.Exit(1)
		}
		opts := mcpOptions(cfg, mcpReadOnly, mcpAllowProtectedApply)
		server := mcp.NewServerWithOptions(mcpAPIKey, rt, opts)
		switch {
		case opts.ReadOnly:
			fmt.Printf("🔎 Mode: read-only (plan_change, apply_change and run_snapshot disabled)\n")
		case mcp.IsProtectedEnvironment(envName, opts.ProtectedEnvironments) && !opts.AllowProtectedApply:
			fmt.Printf("🛡️  Environment %q is protected: apply_change will be refused\n", envName)
		case opts.AllowProtectedApply:
			fmt.Printf("⚠️  --allow-protected-apply set: agents may apply to protected environments\n")
		}

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
	mcpCmd.Flags().BoolVar(&mcpReadOnly, "read-only", false, "Expose only read tools (no plan_change, apply_change or run_snapshot)")
	mcpCmd.Flags().BoolVar(&mcpAllowProtectedApply, "allow-protected-apply", false, "Allow apply_change against environments in protected_environments (default [\"prod\"])")

	rootCmd.AddCommand(mcpCmd)
}
