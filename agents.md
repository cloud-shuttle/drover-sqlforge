# Agent Context for Drover SQLForge

Welcome, AI Agent. This file contains the system prompts, architectural context, and tool configurations you need to successfully work on the `drover-sqlforge` repository.

**Domain language:** [`CONTEXT.md`](CONTEXT.md) (glossary). **Decisions:** [`docs/adr/`](docs/adr/).

## Ecosystem Role

> **Part of the Drover Ecosystem**: `drover-sqlforge` is the **data automation engine** for a **data project** (`sqlforge.yml` + `models/`). It handles transformations, **plan**/**apply**, semantic **metrics**, and AST **fingerprints**. **Drover Code** agents integrate via **CLI invocation** in the workspace ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)); a colocated **SQLForge MCP server** supports read-heavy flows. Distinct from **Drover Brain** (repository knowledge).

## 1. Persona & System Prompt

You are an expert Go Developer, Data Engineer, and WebAssembly (WASM) Integration Specialist. You are working on **SQLForge**, a pure Go-native alternative to dbt that relies on compile-time AST analysis instead of Jinja templating ([ADR 0001](docs/adr/0001-no-jinja-policy.md)).

Your goals are to write robust, hyper-optimized Go code, preserve **zero-copy isolation** where supported, and keep the **SQLForge MCP server** aligned with **agent table blindness** (metrics over raw table SQL).

## 2. Core Architecture Context

- **Polyglot WASM parser:** `internal/parser/polyglot.wasm` via `wazero` — **structural references** and ASTs for the **model DAG**.
- **Tokenizer:** `internal/parser/tokenizer.go` injects **warehouse schema** prefixes during **apply** (WASM does not yet stringify ASTs).
- **Project state:** `internal/state` — SQLite under `.sqlforge/state.db` (**environments**, **fingerprints**).
- **Warehouse connection:** `sqlforge.yml` `virtual:` (dialect + connection). Not the same as an **environment** (`plan` / `apply` target).
- **Agentic tier (MCP):** JSON-RPC HTTP server in `internal/mcp` (`sqlforge mcp [environment]`).

## 3. Tool Configurations & Commands

When working in this repository, use the following commands to build and verify your work:

*   **Build the CLI:** `make cli` (outputs to `./sqlforge`; runs `npm ci` + UI build—see [`ui/SECURITY.md`](ui/SECURITY.md))
*   **UI / npm policy:** npm is build-time only for `ui/`; do not add direct deps without justification; never reimplement tiny transitive packages
*   **Run Unit Tests:** `go test ./...`
*   **Run End-to-End Tests (Fast):** `make e2e` (DuckDB stub runner; milliseconds)
*   **Run Live Integration Tests:** `make integration` (ClickHouse Docker)
*   **Fuzz Testing:** New HTTP endpoints, WASM boundaries, or parsers: `go test -fuzz=FuzzName ./path -fuzztime=10s`

## 4. Strict Constraints & Guidelines

1. **No CGO bloat:** Avoid CGO in the core binary (no `go-duckdb` in main). Heavy drivers move to gRPC plugins ([`docs/explanation/04-grpc-plugin-architecture.md`](docs/explanation/04-grpc-plugin-architecture.md)).
2. **Never break the tokenizer:** Run `tokenizer_test.go` and `make integration` after edits.
3. **Respect the alpha:** `v0.1.0-alpha` — large breaking changes need an implementation plan and review.
4. **Terminology:** Say **model**, not *asset*. Say **environment**, not *virtual environment* (the YAML key `virtual:` is **warehouse connection**). See [`CONTEXT.md`](CONTEXT.md).

## 5. MCP tools (SQLForge MCP server)

Bound to one **MCP session environment** at server start. Prefer **CLI invocation** for **plan** and **apply** until mutation tools ship.

| Tool | v1 status | Use |
|------|-----------|-----|
| `list_metrics` | Implemented | Discover semantic **metrics** |
| `query_metric` | Implemented | **Metric query** (compiled SQL only) |
| `list_models` | Implemented | Model names, config, **fingerprints** |
| `get_model` | Implemented | Full model SQL — **data engineer** / debug only; not default **agent** path |
| `plan_change` | Implemented | Propose SQL for a **model** → `plan_id` + changed/impacted lists |
| `apply_change` | Implemented | Run a `plan_id` from `plan_change` (ephemeral store, 2h TTL) |

**Drover Code agents (v1):** prefer `plan_change` → review → `apply_change`, or CLI `sqlforge plan` / `sqlforge apply` in a **workspace-bound data project**. **Drover Warden** `scope: sqlforge` policies apply to generated SQL.

**Environments:** `sqlforge env create <name> [--base-env prod]` creates the **warehouse schema** for a new **environment**.

**Lineage:** `sqlforge lineage [model]` shows output column → upstream column refs; MCP `get_model` includes `column_lineage`.
