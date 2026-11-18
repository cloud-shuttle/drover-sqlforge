---
title: "Reference: MCP tools"
description: JSON-RPC methods the SQLForge MCP server registers, their parameters, and what they return.
product: drover-sqlforge
audience:
  - platform-operator
  - agent
doc_type: reference
topics:
  - mcp
  - security
  - data-warehousing
surface: repo-docs
---

# Reference: MCP tools

`sqlforge mcp [env]` serves JSON-RPC at `POST /mcp`. The default bind is `127.0.0.1:8080`. With no environment argument the server uses `dev`. The process loads one environment and does not switch.

```json
{"jsonrpc":"2.0","id":1,"method":"list_models","params":{}}
```

`method` is the tool name. `tools/list` returns the registered tools. A body larger than 1 MiB is rejected.

## Who can call

| Startup | Who is accepted |
|---|---|
| `--local` and no API key | Callers on loopback. The actor is `local`. |
| `SQLFORGE_MCP_API_KEY` or `--api-key` | `Authorization: Bearer <key>`. The actor is `api-key`. `--local=false` requires a key or `mcp.auth.mode: oidc`. |
| `mcp.auth.mode: oidc` | Token in `x-drover-identity-token`, or `mcp.auth.token_header`. See [sqlforge.yml](sqlforge-yml.md). |

`--read-only` does not register the write tools. They are absent from `tools/list` and from dispatch.

Write tools: `plan_change`, `apply_change`, `run_snapshot`.

Read tools: `list_models`, `get_model`, `list_metrics`, `query_metric`, `list_snapshots`.

`apply_change` against a protected environment is refused unless the server was started with `--allow-protected-apply`. The default protected environment is `prod`. A plan can only be applied by a server bound to the environment where it was created.

Every tool call writes one JSON audit line to stderr. The line does not include SQL, a DSN, or a key. `get_model` omits `sql`, `ast_type`, and `ast_value` when the actor kind is `agent`. OIDC callers default to kind `agent` unless the token says `member` or `service`. An API-key or local caller has an empty kind and keeps the SQL.

The `warden` hook is not available in this module. Naming it in `mcp.hooks` stops the server at startup.

## `list_models`

No parameters.

Returns an array of `{name, materialized, fingerprint, config}`. `materialized` is the `@materialized` value, which may be empty when the model does not set it.

## `get_model`

| Field | Required | |
|---|---|---|
| `model_name` | yes | Model file name, without `.sql` |

Returns `name`, `path`, `type`, `config`, `sql`, `dependencies`, `fingerprint`, and `column_lineage` when the parser can extract it. When the model has an AST, the response also has `ast_type` and `ast_value`. Unknown `model_name` is an error.

An agent caller receives the same object without `sql`, `ast_type`, and `ast_value`.

## `list_metrics`

No parameters.

Returns the metrics loaded from `metrics.yml`. Errors when the semantic layer is not loaded.

## `query_metric`

| Field | Required | |
|---|---|---|
| `name` | yes | Metric name |
| `dimensions` | no | Group-by dimension names |

Returns `{metric, dimensions, compiled_sql}`. It does not run the SQL.

## `plan_change`

Write tool.

| Field | Required | |
|---|---|---|
| `model_name` | yes | An existing model |
| `proposed_sql` | yes | Replacement SQL |

`proposed_sql` must be one read-only `SELECT`. A second statement, or SQL that is not a select, is rejected before a plan is stored and before any warehouse call.

Returns:

| Field | |
|---|---|
| `plan_id` | Pass this to `apply_change`. Kept for 2 hours. |
| `environment` | Environment this server is bound to |
| `model_name` | |
| `changed_models` | Models whose fingerprint changed |
| `impacted_models` | Unchanged downstream models |
| `unchanged_count` | |

The model must already exist. This tool does not create a model file.

## `apply_change`

Write tool.

| Field | Required | |
|---|---|---|
| `plan_id` | yes | From `plan_change` |

Runs that plan in the server's environment. A plan from another environment is refused. A protected environment is refused without `--allow-protected-apply`. The stored SQL is checked again before it runs. An unknown or expired `plan_id` is an error. A successful apply deletes the stored plan.

Returns `plan_id`, `environment`, and `status`. `status` is `noop` when nothing changed, with `message` set to `nothing to apply`. `status` is `applied` when the warehouse ran, and `applied` is the count of changed plus impacted models.

## `list_snapshots`

No parameters.

Returns the snapshot definitions under `snapshots/`. Each item has `Name`, `Path`, `SQL`, and `Config`. The handler returns the Go struct, so the JSON keys keep those capitals.

## `run_snapshot`

Write tool.

| Field | Required | |
|---|---|---|
| `snapshot_name` | yes | File name under `snapshots/`, without `.sql` |

Runs that snapshot in the server's environment. Returns one result: `Name`, `Initial` (true when no prior snapshot state existed), and `Error` (null when it succeeded). An unknown name is an error and runs nothing.
