package mcp

import "context"

// ToolCall is the dependency-free view of one MCP tool invocation.
type ToolCall struct {
	Tool        string
	PolicyTool  string
	Args        map[string]any
	Environment string
	Actor       Actor
	RequestID   string
}

// Verdict is a hook decision. Action is allow, deny, redact, or require_approval.
type Verdict struct {
	Action   string
	Reason   string
	PolicyID string
	Modified any
}

// ToolHook is called around every tool invocation. Implementations must not
// send arguments or results off-box.
type ToolHook interface {
	PreTool(ctx context.Context, call *ToolCall) Verdict
	PostTool(ctx context.Context, call *ToolCall, result any) Verdict
}

// HookChain runs hooks in order. The first verdict that is not allow wins.
// A panic is a deny, so a broken hook cannot fail open.
type HookChain struct {
	hooks []ToolHook
}

func allowVerdict() Verdict {
	return Verdict{Action: "allow"}
}

func (c *HookChain) PreTool(ctx context.Context, call *ToolCall) (v Verdict) {
	defer func() {
		if recover() != nil {
			v = Verdict{Action: "deny", Reason: "hook failed closed", PolicyID: "hook_panic"}
		}
	}()
	return firstVerdict(c, func(h ToolHook) Verdict { return h.PreTool(ctx, call) })
}

func (c *HookChain) PostTool(ctx context.Context, call *ToolCall, result any) (v Verdict) {
	defer func() {
		if recover() != nil {
			v = Verdict{Action: "deny", Reason: "hook failed closed", PolicyID: "hook_panic"}
		}
	}()
	return firstVerdict(c, func(h ToolHook) Verdict { return h.PostTool(ctx, call, result) })
}

func firstVerdict(c *HookChain, call func(ToolHook) Verdict) Verdict {
	if c == nil {
		return allowVerdict()
	}
	for _, hook := range c.hooks {
		verdict := call(hook)
		if verdict.Action != "" && verdict.Action != "allow" {
			return verdict
		}
	}
	return allowVerdict()
}

// redactHook is the default get_model policy: agents do not receive SQL or AST.
type redactHook struct{}

func (redactHook) PreTool(context.Context, *ToolCall) Verdict {
	return allowVerdict()
}

func (redactHook) PostTool(_ context.Context, call *ToolCall, result any) Verdict {
	if call == nil || call.Tool != "get_model" || call.Actor.Kind != "agent" {
		return allowVerdict()
	}
	fields, ok := result.(map[string]interface{})
	if !ok {
		return allowVerdict()
	}
	cleaned := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		if key == "sql" || key == "ast_type" || key == "ast_value" {
			continue
		}
		cleaned[key] = value
	}
	return Verdict{
		Action:   "redact",
		Reason:   "agent get_model omits SQL and AST",
		PolicyID: "sqlforge_get_model_redact",
		Modified: cleaned,
	}
}
