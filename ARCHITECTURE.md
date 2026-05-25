# Architecture

`sqlforge` is a Go-native SQL transformation engine for a **data project** ([`CONTEXT.md`](CONTEXT.md)):

1. **Pure SQL models** — no Jinja; **structural references** via Polyglot WASM ([ADR 0001](docs/adr/0001-no-jinja-policy.md))
2. **Environments** — named plan/apply targets with isolated **warehouse schemas** and **zero-copy isolation** on supported warehouses
3. **Fingerprints** — stable AST + **model config** hashes drive **plan** / **apply**
4. **CGO-free core** — all warehouse drivers live in standalone gRPC plugin binaries; the `sqlforge` binary has zero CGO dependencies

## Components

| Package | Role |
|---------|------|
| `cmd/sqlforge` | Cobra CLI and `sqlforge mcp` entrypoint |
| `cmd/plugins/sqlforge-plugin-*` | Standalone warehouse plugin binaries (gRPC server per dialect) |
| `internal/parser` | Polyglot WASM + tokenizer for **apply**-time SQL |
| `internal/model` | Load `.sql` **models** and `-- @` **model config** |
| `internal/graph` | **Model DAG**, topological sort, **fingerprints** |
| `internal/plan` | **Execution plan**, **apply**, **data quality assertions**, static catalog compiler |
| `internal/virtual` | Runner interface, gRPC plugin client, DDL helpers, incremental merge strategies |
| `internal/state` | **Local state store** (`.sqlforge/state.db`) |
| `internal/semantic` | **Semantic layer** / **metric** compiler |
| `internal/mcp` | **SQLForge MCP server** (JSON-RPC) |
| `internal/config` | **Project manifest** (`sqlforge.yml`) and **warehouse connection** |
| `ui/` | Optional React DAG viewer (npm build-time only; static `dist/` embedded into binary) |

## Plugin Architecture

The core `sqlforge` binary has no warehouse drivers. Each dialect is a standalone gRPC plugin binary spawned on demand:

```
sqlforge plan prod
   └── loads runner_factory.go
       └── exec: sqlforge-plugin-clickhouse  (unix socket, gRPC)
           └── virtual.Runner interface over protobuf
```

The factory (`internal/virtual/runner_factory.go`) resolves the binary by name (`sqlforge-plugin-<dialect>`) from `$PATH` or the same directory as the `sqlforge` binary. The DSN is passed via `SQLFORGE_PLUGIN_DSN` environment variable.

See [`docs/explanation/04-grpc-plugin-architecture.md`](docs/explanation/04-grpc-plugin-architecture.md) for the full design rationale.

## Plan and apply

1. Load **models** and optional **metrics** (`materialize: true` injects **derived models**).
2. **Plan** — diff **fingerprints** vs **local state store** → **changed**, **impacted**, **unchanged** models.
3. **Apply** — pre-create schemas sequentially; materialise models concurrently in dependency order; update state.
4. **Test** — run column, relationship, and singular SQL assertions against the applied models.

## Static Data Catalog

`sqlforge docs generate [env]` compiles the full model DAG, column-level lineage, semantic metrics, data quality tests, and model configs into a single self-contained HTML file (`target/index.html`). The catalog embeds all data as an inline JSON payload (`window.SQLFORGE_CATALOG`) so it works correctly under the `file://` protocol without any CORS errors.

## Agents and CI

- **Drover Code:** primary integration is **CLI invocation** in the repo ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).
- **MCP server:** `sqlforge mcp [env]` exposes read and mutation tools for agent flows.
- **CI:** **preview environments** (`pr_*`) via GitHub Action ([ADR 0003](docs/adr/0003-preview-environment-ci.md)).

## Further reading

- [ROADMAP.md](ROADMAP.md)
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [docs/explanation/](docs/explanation/)
- [docs/adr/](docs/adr/)
