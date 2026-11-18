//go:build warden

package mcp

import "fmt"

// newWardenHook refuses to pretend the private module is linked.
// Integrate the real adapter from drover-code, which can import drover-warden.
func newWardenHook() (ToolHook, error) {
	return nil, fmt.Errorf("built with -tags warden, but drover-warden is not linked into this public module; integrate WardenHook from drover-code")
}
