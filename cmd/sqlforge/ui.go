package main

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/api"
	"github.com/cloud-shuttle/drover-sqlforge/ui"
	"github.com/spf13/cobra"
)

var (
	uiPortFlag string
	uiHostFlag string
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
		rt, err := loadRuntime(envName)
		if err != nil {
			fmt.Printf("Failed to load project: %v\n", err)
			os.Exit(1)
		}
		defer rt.Close()

		env, err := rt.StateMgr.GetOrCreateEnv(envName, "prod")
		if err != nil {
			fmt.Printf("Failed to resolve environment: %v\n", err)
			os.Exit(1)
		}

		distFS, err := fs.Sub(ui.Assets, "dist")
		if err != nil {
			fmt.Printf("Error loading embedded UI (Did you run 'npm run build' in ui/?): %v\n", err)
			os.Exit(1)
		}

		handler := newUIHandler(rt.DAG, rt.VMgr.Runner(), env.Schema, distFS, uiHostFlag)

		port := "8080"
		if uiPortFlag != "" {
			port = uiPortFlag
		}
		bind := net.JoinHostPort(uiHostFlag, port)
		if !api.IsLoopbackHost(uiHostFlag) {
			fmt.Printf("⚠️  Binding the Web GUI to %s exposes model SQL and warehouse previews to the network.\n", uiHostFlag)
		}
		fmt.Printf("\n🚀 SQLForge Web GUI running at http://%s\n", bind)

		srv := &http.Server{
			Addr:              bind,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      2 * time.Minute,
			IdleTimeout:       2 * time.Minute,
		}
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	uiCmd.Flags().StringVarP(&uiPortFlag, "port", "p", "8080", "Port to run the SQLForge Web GUI on")
	uiCmd.Flags().StringVar(&uiHostFlag, "host", "127.0.0.1", "Interface to bind (non-loopback exposes the GUI to the network)")
	rootCmd.AddCommand(uiCmd)
}
