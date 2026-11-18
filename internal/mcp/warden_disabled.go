//go:build !warden

package mcp

import "fmt"

func newWardenHook() (ToolHook, error) {
	return nil, fmt.Errorf("mcp hook %q requires a binary built with -tags warden; the public module does not import drover-warden", "warden")
}
