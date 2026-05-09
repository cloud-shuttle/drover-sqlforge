package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/drover-org/drover-sqlforge/internal/api"
	"github.com/drover-org/drover-sqlforge/ui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui [environment]",
	Short: "Launch the SQLForge Web GUI",
	Run: func(cmd *cobra.Command, args []string) {
		envName := "dev"
		if len(args) > 0 {
			envName = args[0]
		}

		fmt.Printf("Loading DAG for environment: %s\n", envName)
		_, dag, stateMgr, virtualMgr, _, err := runPipeline(envName)
		if err != nil {
			fmt.Printf("Failed to load project: %v\n", err)
			os.Exit(1)
		}

		env, err := stateMgr.GetOrCreateEnv(envName, "prod")
		if err != nil {
			fmt.Printf("Failed to resolve environment: %v\n", err)
			os.Exit(1)
		}

		// 1. API routes
		http.HandleFunc("/api/dag", api.ServeDAG(dag))
		http.HandleFunc("/api/models/", func(w http.ResponseWriter, r *http.Request) {
			// Basic router for /api/models/...
			if len(r.URL.Path) > 12 && r.URL.Path[len(r.URL.Path)-8:] == "/preview" {
				api.ServeModelPreview(virtualMgr.Runner(), env.Schema)(w, r)
			} else {
				api.ServeModelDetails(dag)(w, r)
			}
		})

		// 2. Serve embedded UI
		distFS, err := fs.Sub(ui.Assets, "dist")
		if err != nil {
			fmt.Printf("Error loading embedded UI (Did you run 'npm run build' in ui/?): %v\n", err)
			os.Exit(1)
		}

		http.Handle("/", http.FileServer(http.FS(distFS)))

		port := "8080"
		fmt.Printf("\n🚀 SQLForge Web GUI running at http://localhost:%s\n", port)

		err = http.ListenAndServe(":"+port, nil)
		if err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
