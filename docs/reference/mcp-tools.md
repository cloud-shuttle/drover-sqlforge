---
title: "Reference: MCP tools and tool policy"
description: The tools the SQLForge MCP server exposes, which are read-only, and how read-only mode and protected environments restrict them.
product: drover-sqlforge
audience:
  - member
  - agent
doc_type: reference
topics:
  - mcp
  - governance-policy
surface: repo-docs
---

# Reference: MCP tools and tool policy

`sqlforge mcp [environment]` serves JSON-RPC 2.0 at `/mcp`, bound to one environment (default `dev`). `tools/list` returns the tools registered for the current mode.

## Tools

| Tool | Parameters | Kind | What it does |
|---|---|---|---|
| `list_models` | none | read | All models with metadata and fingerprints |
| `get_model` | `model_name` | read | Model SQL, AST summary and column lineage |
| `list_metrics` | none | read | Semantic layer metrics |
| `query_metric` | `name`, `dimensions` | read | Compiles a metric to SQL (does not execute it) |
| `list_snapshots` | none | read | Snapshot definitions in `snapshots/` |
| `plan_change` | `model_name`, `proposed_sql` | mutating | Validates a single read-only `SELECT`, builds a plan and returns a `plan_id` |
| `apply_change` | `plan_id` | mutating | Executes a stored plan against the warehouse |
| `run_snapshot` | `snapshot_name` | mutating | Runs a snapshot, writing snapshot tables |

## Read-only mode

```bash
sqlforge mcp prod --read-only
```

Only the read tools are registered. `plan_change`, `apply_change` and `run_snapshot` are missing from `tools/list`, and calling them returns JSON-RPC `-32601 Method not found`. Use this for agent analytics sessions that never need to change models.

## Protected environments

`apply_change` is refused in any environment listed in `protected_environments` in `sqlforge.yml`:

```yaml
protected_environments: [prod, finance]   # default when unset: [prod]
```

- Matching ignores case, because unquoted schema names fold case on most warehouses.
- `protected_environments: []` protects nothing.
- `plan_change` still works in a protected environment, so an agent can propose and review a change. A person then applies it with `sqlforge apply`, or an operator restarts the server with `--allow-protected-apply`.

```bash
sqlforge mcp prod --allow-protected-apply   # explicit opt-in; prints a warning at startup
```

## Plan binding

Each `plan_id` records the environment it was planned for. `apply_change` rejects and discards a plan whose environment doesn't match the server's environment. Plans also expire after 2 hours and are discarded once applied.

## Not covered yet

`run_snapshot` is not subject to `protected_environments`. Use `--read-only` if agents must not write snapshot tables. Caller identity, Drover Warden hooks and an audit log are tracked in the roadmap epic (#65).
