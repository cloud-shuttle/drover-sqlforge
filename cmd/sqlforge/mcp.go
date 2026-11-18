package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/config"
	"github.com/cloud-shuttle/drover-sqlforge/internal/mcp"
	"github.com/spf13/cobra"
)

// mcpAPIKeyEnv is the preferred way to pass the MCP API key: flags are
// visible in process listings and shell history.
const mcpAPIKeyEnv = "SQLFORGE_MCP_API_KEY"

var (
	mcpPort                string
	mcpLocal               bool
	mcpAPIKey              string
	mcpReadOnly            bool
	mcpAllowProtectedApply bool
)

// resolveMCPAPIKey prefers the --api-key flag when set, else the environment.
func resolveMCPAPIKey(flagValue string, getenv func(string) string) string {
	if flagValue != "" {
		return flagValue
	}
	return getenv(mcpAPIKeyEnv)
}

func mcpBindHost(local bool) string {
	if local {
		return "127.0.0.1"
	}
	return "0.0.0.0"
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

		apiKey := resolveMCPAPIKey(mcpAPIKey, os.Getenv)
		host := mcpBindHost(mcpLocal)
		cfg, err := config.LoadConfig(".")
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}
		if err := mcp.ValidateBindMode(host, apiKey, cfg.MCP.Auth.Mode == "oidc"); err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Loading context for environment: %s\n", envName)
		rt, err := loadRuntime(envName)
		if err != nil {
			fmt.Printf("Failed to load project context: %v\n", err)
			os.Exit(1)
		}
		defer rt.Close()

		server := mcp.NewServer(apiKey, rt)
		server.Configure(mcp.ServerConfig{
			ReadOnly:            mcpReadOnly,
			AllowProtectedApply: mcpAllowProtectedApply,
			Protected:           cfg.ProtectedSet(),
		})
		if err := configureGovernance(server, cfg); err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}

		mux := http.NewServeMux()
		mux.Handle("/mcp", server)

		bind := net.JoinHostPort(host, mcpPort)

		fmt.Printf("🚀 SQLForge MCP Server running at http://%s/mcp\n", bind)
		if apiKey != "" {
			fmt.Printf("🔒 Authentication: Enabled (API Key required)\n")
		} else {
			fmt.Printf("⚠️  Authentication: Disabled (loopback only)\n")
		}
		if mcpReadOnly {
			fmt.Printf("Mode: read-only (plan_change, apply_change, and run_snapshot are not registered)\n")
		} else if mcpAllowProtectedApply {
			fmt.Printf("Mode: apply_change allowed against protected environments\n")
		} else {
			fmt.Printf("Mode: apply_change refused for protected environments (default: prod; override with --allow-protected-apply)\n")
		}

		srv := &http.Server{
			Addr:              bind,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      10 * time.Minute, // apply_change can run long warehouse DDL
			IdleTimeout:       2 * time.Minute,
		}
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func configureGovernance(server *mcp.Server, cfg *config.Config) error {
	mode := cfg.MCP.Auth.Mode
	if mode == "" {
		mode = "api_key"
	}
	g := mcp.Governance{
		AuthMode:    mode,
		TokenHeader: cfg.MCP.Auth.TokenHeader,
		HookNames:   cfg.MCP.Hooks,
	}
	if mode == "oidc" {
		verifier, err := mcp.NewOIDCVerifier(cfg.MCP.Auth.Issuer, cfg.MCP.Auth.Audience, cfg.MCP.Auth.RequiredOrg, nil)
		if err != nil {
			return err
		}
		g.Verifier = verifier
	}
	return server.UseGovernance(g)
}

func init() {
	mcpCmd.Flags().StringVar(&mcpPort, "port", "8080", "Port to bind the MCP server to")
	mcpCmd.Flags().BoolVar(&mcpLocal, "local", true, "Bind to 127.0.0.1 instead of 0.0.0.0 (non-local requires an API key)")
	mcpCmd.Flags().StringVar(&mcpAPIKey, "api-key", "", "API key required for access (prefer the "+mcpAPIKeyEnv+" environment variable)")
	mcpCmd.Flags().BoolVar(&mcpReadOnly, "read-only", false, "Register only read tools (omit plan_change, apply_change, and run_snapshot)")
	mcpCmd.Flags().BoolVar(&mcpAllowProtectedApply, "allow-protected-apply", false, "Allow apply_change against protected_environments (default protected: prod)")

	rootCmd.AddCommand(mcpCmd)
}
