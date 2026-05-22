package main

import (
	"context"
	"fmt"
	"os"

	"github.com/drover-org/drover-sqlforge/internal/config"
	"github.com/drover-org/drover-sqlforge/internal/snapshot"
	"github.com/drover-org/drover-sqlforge/internal/state"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [environment] [snapshot_name...]",
	Short: "Run historized snapshots (SCD Type 2)",
	Long: `Apply historized snapshots from the snapshots/ directory into an environment.

Snapshot SQL files use -- @ config lines:
  -- @strategy: timestamp (default)
  -- @unique_key: id
  -- @updated_at: updated_at

See docs/adr/0004-historized-snapshot.md.`,
	Run: runSnapshot,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}

func runSnapshot(cmd *cobra.Command, args []string) {
	envName := "dev"
	var names []string
	if len(args) > 0 {
		envName = args[0]
		names = args[1:]
	}

	defs, err := snapshot.LoadSnapshots("snapshots")
	if err != nil {
		fmt.Printf("Error loading snapshots: %v\n", err)
		os.Exit(1)
	}
	if len(defs) == 0 {
		fmt.Println("No snapshots found in snapshots/")
		return
	}

	defs, err = snapshot.FilterByNames(defs, names)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	stateMgr, err := state.NewManager(".")
	if err != nil {
		fmt.Printf("Error setting up state: %v\n", err)
		os.Exit(1)
	}

	var runner virtual.Runner
	cfg, err := config.LoadConfig(".")
	if err != nil {
		fmt.Printf("Warning: could not load config (%v). Using ClickHouse stub runner.\n", err)
		runner, _ = virtual.NewRunner("clickhouse", "")
	} else {
		runner, err = virtual.NewRunner(cfg.Virtual.Dialect, cfg.Virtual.Connection)
		if err != nil {
			fmt.Printf("Error setting up runner: %v\n", err)
			os.Exit(1)
		}
	}

	vMgr := virtual.NewManager(runner, stateMgr)
	env, err := stateMgr.GetOrCreateEnv(envName, "prod")
	if err != nil {
		fmt.Printf("Error getting environment: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running %d snapshot(s) in environment %s\n", len(defs), envName)
	ctx := context.Background()
	results, err := snapshot.Apply(ctx, defs, env, vMgr, stateMgr)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var failed int
	for _, r := range results {
		if r.Error != nil {
			failed++
			fmt.Printf("  %s: failed — %v\n", r.Name, r.Error)
			continue
		}
		kind := "incremental run"
		if r.Initial {
			kind = "initial build"
		}
		fmt.Printf("  %s: ok (%s)\n", r.Name, kind)
	}

	if failed > 0 {
		os.Exit(1)
	}
	fmt.Println("Snapshots completed successfully.")
}
