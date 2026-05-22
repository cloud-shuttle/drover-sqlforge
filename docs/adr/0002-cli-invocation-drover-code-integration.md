---
title: "ADR 0002: CLI invocation as Drover Code integration"
description: sqlforge plan/apply as the v1 integration surface for Drover Code agents.
product: drover-sqlforge
audience:
  - platform-operator
  - agent
doc_type: adr
topics:
  - data-warehousing
  - agent-jobs
surface: repo-docs
---

# ADR 0002: CLI invocation as v1 Drover Code integration

**Status:** Accepted  
**Date:** 2026-05-22  
**Contexts:** [`CONTEXT.md`](../../CONTEXT.md), [`drover-code/CONTEXT.md`](../../../drover-code/CONTEXT.md), [`CONTEXT-MAP.md`](../../../CONTEXT-MAP.md)

## Context

The Drover ecosystem describes **agents** using **Drover SQLForge** for warehouse schema and model work ([`DROVER_ECOSYSTEM.md`](../../../DROVER_ECOSYSTEM.md) step 9). SQLForge also ships an HTTP **SQLForge MCP server** with tools such as `list_metrics`, `query_metric`, and deferred `plan_change` / `apply_change`.

Integration options for **Drover Code** (BYOC or hosted **agent jobs**):

1. **CLI invocation** — agent runs `sqlforge plan` / `sqlforge apply` / `sqlforge query` as subprocesses in the checked-out Git tree.
2. **Sidecar MCP** — **SQLForge MCP server** colocated with the agent; reached via **Drover Gateway** MCP routing.
3. **Hosted SQLForge tenancy** — platform maps **customers** to warehouses and credentials (not v1).
4. **Muster-resolved MCP only** — agents discover SQLForge exclusively through **Drover Muster** (deferred gate).

Without a decision, implementers might block **agent jobs** on unfinished MCP mutation tools or conflate **Brain MCP** (repository knowledge) with SQLForge (metrics/warehouse).

## Decision

### 1. Workspace-bound activation

SQLForge runs only when the agent’s checkout is a **workspace-bound data project**: `sqlforge.yml` at the **project root** (or `config_dir` in CI). No platform-provisioned SQLForge service in v1.

### 2. Primary path: CLI invocation

**Drover Code** integrates with SQLForge in v1 via **CLI invocation**:

| Operation | Command | Mutation |
|-----------|---------|----------|
| Diff | `sqlforge plan [environment]` | No |
| Deploy | `sqlforge apply [environment]` | Yes |
| Analytics SQL | `sqlforge query [metric] [environment]` | No (compile only) |

- Same semantics as a local **data engineer**; non-TTY **apply** skips TUI (CI-friendly).
- **Agents** use bash/tooling in `/workspace`; **Drover Warden** `scope: sqlforge` policies apply to generated SQL and destructive operations.
- **Apply** is never implicit on **fingerprint** change; human or CI gate required.

### 3. Secondary path: sidecar MCP (read-heavy)

**Sidecar MCP** is supported for read-heavy flows when Gateway routing is configured:

- **Agent-safe MCP tools (v1):** `list_metrics`, `query_metric`; optionally `list_models` (metadata only).
- **`get_model`:** restricted to **data engineer** / debugging; not default **agent** path.
- **`plan_change` / `apply_change`:** implemented with an ephemeral in-memory plan store (`plan_id`, 2h TTL); agents should review diffs before calling `apply_change`.

### 4. Deferred platform integration

| Capability | Status |
|------------|--------|
| **Muster** tool discovery for SQLForge MCP | Deferred (Muster integration gate) |
| **Guard** approval for destructive **apply** | Deferred (Guard gate) |
| **Hosted SQLForge tenancy** | Deferred |
| **Brain vs SQLForge** | Always separate MCP servers — Brain for code knowledge, SQLForge for **metrics** / **model DAG** / warehouse **plan** |

## Consequences

**Positive**

- **Agent jobs** can use SQLForge today without waiting for MCP mutation APIs.
- Clear split: Brain answers “how is auth implemented”; SQLForge answers “what is `daily_revenue`”.
- CI and local dev share one invocation model.

**Negative**

- Subprocess CLI lacks fine-grained MCP streaming until sidecar MCP matures.
- Operators must inject warehouse credentials via `.env` / secrets in the workspace—no Cloud-managed warehouse binding yet.
- Dual documentation burden (CLI + MCP) until deferred tools ship.

**Follow-ups**

- Implement `plan_change` / `apply_change` with explicit approval handshake (align with **CI apply gate** in ADR 0003).
- Register SQLForge MCP in **Muster** when integration gate opens.
- ~~Document example `drover-code` skill/preset for **workspace-bound data project** detection.~~ Done: `drover-code/internal/integrations/sqlforge`, system injection in `internal/config/loader.go`, `docs/how-to/sqlforge-from-drover-code.md`, `.drover/commands/sqlforge-plan.md` / `sqlforge-apply.md`.

## References

- Glossary: [`CONTEXT.md`](../../CONTEXT.md) — § Ecosystem integration, § Agent integration (MCP)
- ADR 0003: Preview environment CI pattern
- Drover Code: [`drover-code/CONTEXT.md`](../../../drover-code/CONTEXT.md)
- Warden sqlforge policies: [`drover-warden/beads/policies.jsonl`](../../../drover-warden/beads/policies.jsonl)
