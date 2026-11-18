---
title: "ADR 0007: MCP governance — Gateway identity, Warden hook, audit log"
description: Design for verified caller identity, a pluggable pre/post tool hook for Drover Warden scope sqlforge, a structured audit log, and Gateway virtual keys for the ai client.
product: drover-sqlforge
audience: platform-operator
doc_type: adr
topics:
  - security
  - mcp
  - identity-auth
  - governance-policy
  - observability
  - llm-routing
surface: repo-docs
---

# 7. MCP governance: Gateway identity, Warden hook, and audit log

Date: 2026-11-16

## Status

Accepted. Implemented in the MCP server: actor, audit log, OIDC verifier, tool hook, and Gateway virtual key. The Warden adapter is a build-tag stub and does not import drover-warden.

## Context

[CONTEXT.md](../../CONTEXT.md) says **agents** reach the **sidecar MCP** server through **Drover Gateway**, and that agent constraints come from **Drover Warden** (`scope: sqlforge`). Today SQLForge has no hook for either:

- **Auth is one static key.** `sqlforge mcp` accepts a single shared bearer key (`SQLFORGE_MCP_API_KEY`; #52 hardens how it is passed). There is no caller identity, so every request is equally trusted.
- **No audit trail.** Nothing records who listed, planned, applied, or snapshotted what.
- **The `ai` client bypasses the Gateway.** `sqlforge ai explain` posts directly to an Ollama-style endpoint (`ai.endpoint`, default `http://localhost:11434`), so LLM spend and model policy skip Gateway virtual keys and budgets.

The surrounding platform boundaries are fixed elsewhere, and this ADR must not move them:

| Layer | Owns | SQLForge's relationship |
|-------|------|-------------------------|
| **Drover Gateway** | Virtual keys (VK), budgets, MCP routing, request logging. Registered MCP backends receive caller headers listed in that client's `allowed_extra_headers` (the documented "MCP credential forwarding" pattern). | SQLForge is a registered MCP backend. It **verifies** what Gateway forwards and never trusts it blindly. |
| **Drover Guard** | ReBAC (SpiceDB), OPA, capability binding, HITL, decision audit. | SQLForge does not reimplement ReBAC or tenancy. Authorization of *who may call which tool* stays in Guard and Gateway. |
| **Drover Warden** | Semantic safety on tool input and output (JSONL "Beads" policies, `CheckInput` / `CheckOutput` / `CheckAction`). A pure-Go, in-process library, not a network service. Hosted enforcement runs **inside drover-code on the worker**. | SQLForge exposes a hook so the same policies can also run server-side (BYOC, sidecar, self-hosted). It is defense in depth, not a second policy engine. |

## Decision

### 1. Caller identity: optional OIDC mode

Add a second auth mode to `sqlforge mcp`. The static key remains the default, and the loopback-only no-auth mode is unchanged.

```yaml
# sqlforge.yml
mcp:
  auth:
    mode: oidc            # api_key (default) | oidc
    issuer: https://<zitadel-instance>            # exact match on `iss`
    audience: <sqlforge-mcp project/client id>    # must appear in `aud`
    token_header: x-drover-identity-token         # header Gateway forwards (allowed_extra_headers)
    jwks_cache_ttl: 10m
    required_org: ""      # optional: pin to one Zitadel org id
```

Verification rules. Each one is mandatory, and the request is rejected (401) on any failure:

- **Algorithm and key:**
  - Signature verified against the issuer's JWKS, discovered via `/.well-known/openid-configuration`, with keys cached and refreshed on an unknown `kid`.
  - Algorithm allow-list: `RS256` and `ES256`. `none` and HMAC are rejected.
- **Claims:**
  - **`iss` must equal the configured issuer exactly.**
  - **`aud` must contain the configured audience.** A token minted for another Gateway backend or another project is rejected even if it is validly signed.
  - `exp` and `nbf` are enforced with at most 60s of clock skew. `iat` is required.
  - If `required_org` is set, `urn:zitadel:iam:org:id` must match it.
- **Forwarded headers are never trusted on their own.** SQLForge ignores Gateway-derived convenience headers such as `X-User-ID`, `X-Org-ID` and `X-User-Roles`. Identity comes only from the verified token. A missing or invalid token fails closed. It never falls back to the static key.
- **Transport:** OIDC mode requires TLS or a loopback/Unix-socket hop from the Gateway sidecar. The existing `ValidateBind` rule extends to "non-loopback requires `api_key` or `oidc`".

The verified token yields an **actor**, which is passed through `context.Context` to every tool handler, the hook, and the audit log:

```go
type Actor struct {
    Subject string   // `sub`
    OrgID   string   // `urn:zitadel:iam:org:id`
    Kind    string   // "member" | "agent" | "service" (from a project role or claim; default "agent")
    Roles   []string // `urn:zitadel:iam:org:project:roles` keys (informational; not used for ReBAC)
    Via     string   // "oidc" | "api_key" | "local"
}
```

In `api_key` mode the actor is `{Subject: "api-key", Via: "api_key"}`. In local mode it is `{Subject: "local", Via: "local"}`. Roles are recorded for audit only. **Authorization decisions stay in Guard and Gateway**: SQLForge does not grow a role matrix or tenancy model, consistent with CONTEXT.md (v1 has no SQLForge tenancy).

Gateway setup is configuration only:

- Register SQLForge MCP as an MCP client with `token_header` listed in `allowed_extra_headers`.
- The worker sends the member's or agent's Zitadel token in that header.

No Gateway code or auth-mode changes are required. In particular, Gateway's enterprise sentinel stays unset, and virtual-key isolation and prompt protection are untouched.

### 2. Pluggable tool hook (Warden `scope: sqlforge`)

`internal/mcp` gets a small, dependency-free interface, called around every `tools/call`:

```go
// internal/mcp/hook.go
type ToolCall struct {
    Tool        string         // MCP tool name, e.g. "apply_change"
    PolicyTool  string         // Warden-facing name: "sqlforge_" + Tool (see below)
    Args        map[string]any // decoded arguments (mutable copy)
    Environment string         // MCP session environment
    Actor       Actor
    RequestID   string
}

type Verdict struct {
    Action   string // "allow" | "deny" | "redact" | "require_approval"
    Reason   string // safe to return to the caller and to log
    PolicyID string
    Modified any    // rewritten args (pre) or redacted result (post) when Action == "redact"
}

type ToolHook interface {
    PreTool(ctx context.Context, call *ToolCall) Verdict
    PostTool(ctx context.Context, call *ToolCall, result any) Verdict
}
```

- **Fail closed:**
  - `deny` and `require_approval` return a JSON-RPC error that carries `Reason` and `PolicyID`. The tool does not run.
  - A hook panic or error is a `deny`.
  - `require_approval` is surfaced as a distinct error code, so that drover-code or Guard can route it to HITL. SQLForge never self-approves.
- **Ordering:** auth (§1), then `PreTool`, then the tool, then `PostTool`, then the audit (§3). Read-only mode (#55) and the SELECT-only guard (#53) still apply inside the tool. The hook can only make a call *stricter*.
- **Hooks are chainable.** `mcp.hooks: [warden, ...]` in config; the first non-`allow` verdict wins.
- **Warden adapter:**
  - Warden is a private module, and this repository is public with zero-dependency builds. So the adapter (`WardenHook`, which maps `ToolCall` to `warden.GuardRequest{ToolCall: {ToolName: PolicyTool, Args}, TenantID: Actor.OrgID}` and calls `CheckAction` / `CheckOutput`) ships behind a build tag (`-tags warden`) or in the drover-code integration.
  - The default build carries only the interface and a no-op hook.
  - No Warden code is vendored here.
- **Tool naming:** Warden `scope: sqlforge` Beads match tools by name, e.g. the existing destructive-SQL policy lists `sqlforge_apply`. Implicit scope matching keys on `sql` appearing in the tool name, and SQLForge's MCP names (`plan_change`, `apply_change`, `get_model`) do not contain it.
  - The adapter therefore sends `PolicyTool = "sqlforge_" + Tool`, e.g. `sqlforge_apply_change`.
  - A follow-up in drover-warden aligns the Bead tool lists (`sqlforge_apply` becomes `sqlforge_apply_change`, plus `sqlforge_plan_change` and `sqlforge_run_snapshot`). That change is tracked there, not in this repo.
- **`get_model` redaction for agents:**
  - The built-in default policy (shipped with the hook, overridable) redacts the `SQL` and AST fields of `get_model` results when `Actor.Kind == "agent"`. Name, description, dependencies and config stay.
  - Agents are steered to `query_metric` and the semantic tools, per the CONTEXT.md agent data-access rule.
  - Data engineers (`member`) see the full model.
- **What the hook sees:** decoded arguments and results in process. Nothing is sent off-box by SQLForge.

### 3. Structured audit log

One JSON line per `tools/call`, written after `PostTool` (or after the failure that ended the call), to a configurable sink: `stderr` (default), a file, or OTEL logs.

```json
{"ts":"2026-11-16T01:02:03.456Z","event":"sqlforge.mcp.tool_call","request_id":"…",
 "actor":{"sub":"…","org":"…","kind":"agent","via":"oidc"},
 "tool":"apply_change","environment":"dev","plan_id":"pln_…",
 "outcome":"allowed|denied|redacted|require_approval|error",
 "policy_id":"pol-021","error_class":"plan_drift","duration_ms":182,
 "models":["stg_events"],"rows_affected":null}
```

Field rules:

- **Always present:** `ts` (UTC, RFC 3339 with milliseconds), `actor`, `tool`, `environment`, `outcome`, `duration_ms`. `plan_id` appears for `plan_change` and `apply_change`. `policy_id` appears when a hook returned a non-`allow` verdict.
- **Never logged:**
  - SQL text or literals, model SQL, `query_metric` filter values, or result rows
  - tokens, API keys, virtual keys, DSNs or connection strings
  - raw tool arguments

  Only allow-listed, structural fields are emitted: model names, metric names, environment and plan ids. Errors are logged as a class (`plan_drift`, `validation`, `warehouse`, `policy`, `auth`) and never as the raw message, which can echo SQL.
- **Auth failures** (401s) are logged with `outcome: "auth_failed"` and the failure reason class (expired, wrong audience, wrong issuer, bad signature). Token contents are not logged.
- **Field names line up with Gateway logging and the `x-bf-dim-*` trace dimensions**, so HyperDX/ClickHouse can join a tool call to the agent job. If a `x-bf-dim-agent-job-id` header is forwarded and the hook allows it, it is copied as `agent_job_id`.

### 4. `ai` client through Gateway virtual keys

```yaml
ai:
  provider: gateway          # ollama (default, unchanged) | gateway
  endpoint: https://<gateway>/v1   # OpenAI-compatible
  model: <model allowed by the VK>
```

- **The VK comes only from the environment** (`SQLFORGE_AI_VIRTUAL_KEY`). It is sent as `x-bf-vk` and is never read from `sqlforge.yml`, flags or the repo. `sqlforge ai` refuses to start in `gateway` mode without it.
- **The VK must be the member's own** (per-member or job-scoped) **virtual key**. Docs and the error text say never to use a shared or agent-facing key. SQLForge cannot prove a key is per-member, so the configuration path makes the right choice the only documented one.
- **Requests are tagged** `X-Source: drover-sqlforge` plus any `x-bf-dim-*` dimensions present in the environment. Gateway budgets, model allow-lists and request logging then apply unchanged.
- **Prompt content stays minimal:** model SQL is sent only for `ai explain` on a model the caller already has, and is never logged by SQLForge.

## Consequences

### Positive

- **Identity:** every MCP call has a verified actor, and tokens for other audiences are rejected.
- **Warden:** policies apply to SQLForge tools by name, server-side as well as in the agent loop. `get_model` SQL is withheld from agents by default.
- **Audit:** an append-only, secret-free record answers "who planned or applied what".
- **LLM routing:** `ai` traffic is governed by Gateway VKs and budgets.

### Negative

- **Operational setup:** OIDC mode adds JWKS fetching and a Gateway MCP client registration step.
- **Build split:** the Warden adapter needs a build tag or an external integration, because the public build cannot depend on the private module.
- **Naming coordination:** the `sqlforge_` tool-name prefix needs a matching Bead update in drover-warden.

### Not changing

- No SQLForge tenancy or ReBAC. SpiceDB, Guard, virtual-key isolation and prompt protection remain the authorities.
- Read-only MCP mode (#55) and SELECT-only `plan_change` validation (#53) are unchanged. Hooks can only tighten them.

## Implementation slices (each its own PR under #62)

1. **Actor plumbing and audit log** (api_key and local actors), with tests asserting that no SQL or secrets appear in log lines.
2. **OIDC verifier.** Table tests cover wrong `iss`, wrong `aud`, expired, `none`/HS256, unknown `kid`, and header-only identity; a local JWKS fixture covers key rotation.
3. **`ToolHook` interface**, chain, fail-closed semantics, and the default `get_model` agent redaction.
4. **`WardenHook` adapter** behind `-tags warden`, plus a drover-warden Bead tool-name alignment issue.
5. **`ai` gateway provider** with the env-only VK.
