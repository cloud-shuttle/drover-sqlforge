package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/drover-org/drover-sqlforge/internal/project"
	"github.com/drover-org/drover-sqlforge/internal/snapshot"
)

// registerSnapshotTools registers MCP tools for managing and executing snapshots.
func (r *Registry) registerSnapshotTools(rt *project.Runtime) {
	r.Register(Tool{
		Name:        "list_snapshots",
		Description: "List all snapshot definitions available in the project",
		InputSchema: ToolSchema{
			Type:       "object",
			Properties: map[string]SchemaProperty{},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			dir := "snapshots"
			if rt != nil && rt.ProjectDir != "" {
				dir = filepath.Join(rt.ProjectDir, "snapshots")
			}
			defs, err := snapshot.LoadSnapshots(dir)
			if err != nil {
				return nil, fmt.Errorf("failed to load snapshots: %w", err)
			}
			return defs, nil
		},
	})

	r.Register(Tool{
		Name:        "run_snapshot",
		Description: "Execute a snapshot by name in the current environment",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]SchemaProperty{
				"snapshot_name": {Type: "string", Description: "The name of the snapshot to run"},
			},
			Required: []string{"snapshot_name"},
		},
		Handler: func(ctx context.Context, params []byte) (interface{}, error) {
			if rt == nil || rt.Env == nil {
				return nil, fmt.Errorf("no project runtime available")
			}

			var args struct {
				SnapshotName string `json:"snapshot_name"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("invalid parameters")
			}

			dir := filepath.Join(rt.ProjectDir, "snapshots")
			defs, err := snapshot.LoadSnapshots(dir)
			if err != nil {
				return nil, fmt.Errorf("failed to load snapshots: %w", err)
			}

			filtered, err := snapshot.FilterByNames(defs, []string{args.SnapshotName})
			if err != nil {
				return nil, err
			}

			results, err := snapshot.Apply(ctx, filtered, rt.Env, rt.VMgr, rt.StateMgr)
			if err != nil {
				return nil, fmt.Errorf("snapshot execution failed: %w", err)
			}

			return results, nil
		},
	})
}
