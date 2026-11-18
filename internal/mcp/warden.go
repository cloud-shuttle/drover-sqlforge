package mcp

// WardenRequest is the local shape a Warden adapter would send.
// This module does not import drover-warden.
type WardenRequest struct {
	ToolName string
	Args     map[string]any
	TenantID string
}

// WardenRequestFrom maps a tool call onto that shape.
// PolicyTool is sqlforge_ plus the MCP tool name.
func WardenRequestFrom(call *ToolCall) WardenRequest {
	if call == nil {
		return WardenRequest{}
	}
	return WardenRequest{
		ToolName: call.PolicyTool,
		Args:     call.Args,
		TenantID: call.Actor.OrgID,
	}
}
