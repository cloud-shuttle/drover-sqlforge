# Architecture

`sqlforge` is a Go-native SQL transformation engine for a **data project** ([`CONTEXT.md`](CONTEXT.md)):

1. **Pure SQL models** — no Jinja; **structural references** via Polyglot WASM ([ADR 0001](docs/adr/0001-no-jinja-policy.md))
2. **Environments** — named plan/apply targets with isolated **warehouse schemas** and **zero-copy isolation** on supported warehouses
3. **Fingerprints** — stable AST + **model config** hashes drive **plan** / **apply**

## Components

| Package | Role |
|---------|------|
| `cmd/sqlforge` | Cobra CLI and `sqlforge mcp` entrypoint |
| `internal/parser` | Polyglot WASM + tokenizer for **apply**-time SQL |
| `internal/model` | Load `.sql` **models** and `-- @` **model config** |
| `internal/graph` | **Model DAG**, topological sort, **fingerprints** |
| `internal/plan` | **Execution plan**, **apply**, **data quality assertions** |
| `internal/virtual` | Dialect runners (DDL, **incremental merge**, streaming on ClickHouse) |
| `internal/state` | **Local state store** (`.sqlforge/state.db`) |
| `internal/semantic` | **Semantic layer** / **metric** compiler |
| `internal/mcp` | **SQLForge MCP server** (JSON-RPC) |
| `internal/config` | **Project manifest** (`sqlforge.yml`) and **warehouse connection** |

## Plan and apply

1. Load **models** and optional **metrics** (`materialize: true` injects **derived models**).
2. **Plan** — diff **fingerprints** vs **local state store** → **changed**, **impacted**, **unchanged** models.
3. **Apply** — materialize in dependency order; update state.

## Agents and CI

- **Drover Code:** primary integration is **CLI invocation** in the repo ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).
- **CI:** **preview environments** (`pr_*`) via GitHub Action ([ADR 0003](docs/adr/0003-preview-environment-ci.md)).

## Further reading

- [ROADMAP.md](ROADMAP.md)
- [docs/explanation/](docs/explanation/)
- [docs/adr/](docs/adr/)
