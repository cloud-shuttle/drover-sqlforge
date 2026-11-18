package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, req Request, actor Actor) {
	started := time.Now()
	if req.Method == "tools/list" {
		s.invoke(w, r, req, actor, func(ctx context.Context, _ json.RawMessage) (any, error) {
			return s.Registry.ListTools(), nil
		}, started)
		return
	}
	tool, ok := s.Registry.Get(req.Method)
	if !ok {
		s.auditOutcome(r, actor, req.Method, started, "error", "not_found", "")
		sendError(w, req.ID, MethodNotFound, "Method not found")
		return
	}
	s.invoke(w, r, req, actor, func(ctx context.Context, params json.RawMessage) (any, error) {
		return tool.Handler(ctx, params)
	}, started)
}

func (s *Server) invoke(w http.ResponseWriter, r *http.Request, req Request, actor Actor, run func(ctx context.Context, params json.RawMessage) (any, error), started time.Time) {
	line := s.baseRecord(r, actor, req.Method)
	args, err := decodeArgs(req.Params)
	if err != nil {
		s.finishError(w, req.ID, line, started, InvalidParams, "invalid_params", "invalid parameters")
		return
	}
	call := s.toolCall(r, actor, req.Method, args)
	verdict := s.preTool(r.Context(), call)
	if blocked(verdict) {
		s.finishBlocked(w, req.ID, line, call, started, verdict)
		return
	}
	params := paramsFromVerdict(req.Params, verdict, call)
	ctx := contextWithActor(r.Context(), actor)
	ctx = withApplyPolicy(ctx, ApplyPolicy{AllowProtectedApply: s.AllowProtectedApply, Protected: s.Protected})
	result, runErr := run(ctx, params)
	if runErr != nil {
		line.PlanID, line.Models = safePlanAndModels(call.Args, nil)
		s.finishError(w, req.ID, line, started, InternalError, errorClass(runErr), runErr.Error())
		return
	}
	post := s.postTool(ctx, call, result)
	if blocked(post) {
		s.finishBlocked(w, req.ID, line, call, started, post)
		return
	}
	if post.Action == "redact" {
		result = post.Modified
		line.Outcome = "redacted"
		line.PolicyID = safeLog(post.PolicyID, 128)
	} else {
		line.Outcome = "allowed"
	}
	line.PlanID, line.Models = safePlanAndModels(call.Args, result)
	line.DurationMS = time.Since(started).Milliseconds()
	s.writeAudit(line)
	sendSuccess(w, req.ID, result)
}

func (s *Server) toolCall(r *http.Request, actor Actor, method string, args map[string]any) *ToolCall {
	return &ToolCall{
		Tool:        method,
		PolicyTool:  "sqlforge_" + method,
		Args:        args,
		Environment: s.sessionEnvironment(),
		Actor:       actor,
		RequestID:   requestID(r),
	}
}

func (s *Server) preTool(ctx context.Context, call *ToolCall) Verdict {
	if s.Hooks == nil {
		return allowVerdict()
	}
	return s.Hooks.PreTool(ctx, call)
}

func (s *Server) postTool(ctx context.Context, call *ToolCall, result any) Verdict {
	if s.Hooks == nil {
		return allowVerdict()
	}
	return s.Hooks.PostTool(ctx, call, result)
}

func blocked(v Verdict) bool {
	return v.Action == "deny" || v.Action == "require_approval"
}

func paramsFromVerdict(raw json.RawMessage, verdict Verdict, call *ToolCall) json.RawMessage {
	if verdict.Action != "redact" {
		return raw
	}
	rewritten, ok := verdict.Modified.(map[string]any)
	if !ok {
		return raw
	}
	encoded, err := json.Marshal(rewritten)
	if err != nil {
		return raw
	}
	call.Args = rewritten
	return encoded
}

func (s *Server) finishBlocked(w http.ResponseWriter, id json.RawMessage, line auditLine, call *ToolCall, started time.Time, verdict Verdict) {
	line.PlanID, line.Models = safePlanAndModels(call.Args, nil)
	line.Outcome = verdict.Action
	line.PolicyID = safeLog(verdict.PolicyID, 128)
	line.ErrorClass = "policy"
	line.DurationMS = time.Since(started).Milliseconds()
	s.writeAudit(line)
	code := InternalError
	if verdict.Action == "require_approval" {
		code = ApprovalRequired
	}
	reason := verdict.Reason
	if reason == "" {
		reason = "denied by policy"
	}
	sendError(w, id, code, reason)
}

func (s *Server) finishError(w http.ResponseWriter, id json.RawMessage, line auditLine, started time.Time, code int, class, message string) {
	line.Outcome = "error"
	line.ErrorClass = class
	line.DurationMS = time.Since(started).Milliseconds()
	s.writeAudit(line)
	sendError(w, id, code, message)
}

func (s *Server) auditOutcome(r *http.Request, actor Actor, tool string, started time.Time, outcome, class, policy string) {
	line := s.baseRecord(r, actor, tool)
	line.Outcome = outcome
	line.ErrorClass = class
	line.PolicyID = safeLog(policy, 128)
	line.DurationMS = time.Since(started).Milliseconds()
	s.writeAudit(line)
}

func decodeArgs(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	return args, nil
}
