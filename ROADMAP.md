# SQLForge Roadmap

> **Live roadmap:** planned and in-progress work is tracked in GitHub issues under [epic #65](https://github.com/cloud-shuttle/drover-sqlforge/issues/65). This file records what has shipped. Items are only ticked when the code exists on `main`. The 2026-11 audit (#60) corrected the earlier entries that claimed features which were never built.

## The Drover gap program

In November 2026 Drover was compared with Vercel eve 1.0. Drover leads on isolation, the model gateway and governance, and trails on the layer people touch. The program closes five gaps across the repos: durable multi-turn sessions (drover-kelpies), a Slack-first channels layer (drover-cloud), unified OpenTelemetry tracing with a run viewer (drover-libs, Cloud console), a TypeScript client SDK (drover-cloud), and an extension bundle format (drover-muster, drover-registry). Four capabilities are deferred to Later: shared multi-person sessions, ambient channel listening, agents editing their own code, and session prewarm and migration.

Program tracking issue: in [cloud-shuttle/drover-cloud](https://github.com/cloud-shuttle/drover-cloud/issues?q=is%3Aissue+label%3Aroadmap+Tracking) (private). Sessions run on DBOS, not Temporal (ADR 0005 in drover-kelpies). Dates and day counts are estimates.

| Milestone | Due (estimate) |
|---|---|
| M0 Truth-up | 27 Nov 2026 |
| M1 Trace spine | 18 Dec 2026 |
| M2 Durable sessions | 5 Feb 2027 |
| M3 Channels and SDK | 5 Mar 2027 |
| M4 Extensions | 2 Apr 2027 |
| Later | after M4 |

SQLForge is outside the five gaps. Its own plan stays in this file and under epic #65; the only program item here is M0 hygiene (status badge and dead links, done in this PR).

## ✅ Phase 1: Foundation & MVP Engine (Complete)
- [x] **Polyglot WASM Parser Integration**: Cross-platform, sub-millisecond AST generation for model extraction.
- [x] **Model Loader & DAG Builder**: Structural reference resolution and chronological dependency tracking.
- [x] **Zero-copy isolation**: Idempotent materializations for non-prod **environments** (ClickHouse `CLONE`, `VIEW`, etc.).
- [x] **State Management**: Embedded SQLite to track model AST fingerprints and environment schemas.
- [x] **Plan / Apply Pipeline**: Topological sorting and execution logic for live cloud deployments.
- [x] **Self-Contained Examples**: The `agentic_retail_2026` mock dataset running seamlessly via ClickHouse generator functions.

## ✅ Phase 2: Advanced Data Infrastructure (Complete)
- [x] **Native Semantic Layer**: Metric parsing (`metrics.yml`) and dialect-agnostic ANSI SQL compiler.
- [x] **Hybrid Materialization**: Dynamic injection of semantic metrics into the execution DAG (`materialize: true`).
- [x] **Property-Based Testing**: Fuzzer validation for the semantic compiler to prevent SQL injection or panics.
- [x] **Multi-Dialect Virtual Runners**: Implement drivers for ClickHouse, DuckDB, Postgres, Snowflake, Databricks, Apache Doris, and VeloDB via the gRPC plugin architecture.
- [x] **Deep Agentic AI Integration**: Advanced query generation and schema validation via lightweight LLMs (expanding on `sqlforge ai explain`).
- [x] **Real-Time / Streaming Engines**: Support for mapping Kafka/NATS topics to virtual envs and orchestrating ClickHouse Materialized Views.

## ✅ Phase 3: Developer Experience & Automation (Complete)
- [x] **End-to-End Testing**: Fully automated isolated execution suite (`make e2e`) validating the CLI, DAG execution, and internal state trackers against mock projects.
- [x] **Agentic Query Gen**: Agents can run `sqlforge query` to safely synthesize new queries via semantic layer primitives without seeing underlying tables.
- [x] **TUI / Web GUI**: Interactive terminal interface or lightweight local web server to visualize the DAG execution plan.
- [x] **Data Quality Testing — Tier 1**: Standard column macros (`not_null`, `unique`, `accepted_values`) evaluated as native Go assertions during `apply`.
- [x] **Data Quality Testing — Tier 2**: Relationship (foreign key) tests (`test_relationship`) and singular custom SQL assertions in `tests/`.
- [x] **Incremental Materializations**: Robust cross-dialect support for complex incremental models (MERGE, UPSERT, Insert/Overwrite) utilizing AST diffing.
- [x] **CI/CD Integrations**: Pre-built GitHub Actions for spinning up isolated, zero-copy preview environments on every PR.

## 🟡 Phase 4: Agentic Tier / MCP Server (Partially complete)
- [x] **MCP Server**: Embed an HTTP server to expose SQLForge as a Model Context Protocol service for autonomous agents.
- [x] **MCP Tool Registry**: Read and mutation tools (`list_models`, `query_metric`, `plan_change`, `apply_change`, etc.); ephemeral plan store for `apply_change` ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).
- [x] **API key auth**: A single static bearer key (`--api-key`). Hardening is in progress in #52.
- [x] **Read-only MCP and protected environments**: `sqlforge mcp --read-only` omits write tools. `protected_environments` defaults to `prod`. Plans apply only in the environment they were created for (#55).
- [ ] **Interactive Execution**: WebSocket endpoints for live plan approval and step-by-step feedback. *Not implemented: the MCP server is plain JSON-RPC over HTTP POST.*
- [ ] **Rate limiting**: *Not implemented.*
- [x] **Request auditing**: a structured audit log per tool call, with caller identity from Drover Gateway (#62). SQL and secrets are not logged. OIDC verification, the tool hook, and Gateway virtual keys ship with it. The Warden adapter stays a build-tag stub.

## ✅ Phase 5: Lineage, Stability & Developer Onboarding (Complete)
- [x] **Column lineage CLI**: `sqlforge lineage [model]` with structural SELECT/FROM parsing.
- [x] **MCP column_lineage**: `get_model` returns per-column upstream refs.
- [x] **env create e2e**: `TestE2EEnvCreate` validates CLI + SQLite state for preview environments.
- [x] **End-to-End Testing Expansion**: Full coverage for incremental workflows, data quality assertions, and CLI integrations.
- [x] **Fuzz Testing Security**: High-throughput property testing on the WASM parser boundary and JSON-RPC MCP server.
- [x] **Onboarding Material**: Comprehensive `CONTRIBUTING.md` created to outline architecture, local environment setup, and compilation.
- [x] **WASM AST Transpilation**: Leverage the polyglot WASM parser to automatically transpile SQL dialects on the fly (e.g., compile Snowflake syntax down to DuckDB for local development) without altering source `.sql` files.

## ✅ Phase 5.5: Static Data Catalog & Plugin Completion (Complete)
- [x] **Static Data Catalog** (`sqlforge docs generate`): Compiles the entire model DAG, column-level lineage, semantic metrics, data quality assertions, and model configs into a single self-contained offline HTML file — zero CORS, shareable via S3 or GitHub Pages.
- [x] **Doris gRPC Plugin** (`sqlforge-plugin-doris`): Full standalone plugin via MySQL wire protocol with Doris-dialect DDL.
- [x] **VeloDB gRPC Plugin** (`sqlforge-plugin-velodb`): Full standalone plugin via MySQL wire protocol with VeloDB-dialect DDL.
- [x] **`make plugins` build target**: Compiles all five warehouse plugin binaries in a single step.
- [x] **Core binary CGO-free**: All warehouse drivers are fully isolated in standalone gRPC plugin binaries; `sqlforge` has zero CGO dependencies.

## 🎯 v0.2.0 Beta — Test Coverage & Stability (Next Milestone)

Before graduating from alpha to beta, we are targeting measurable improvements to unit test coverage in the critical internal packages. These thresholds gate the beta release.

| Package | Current (v0.1.0-alpha) | Target (v0.2.0-beta) |
|---------|------------------------|----------------------|
| `internal/plan` | 85.4% | **≥ 80%** |
| `internal/state` | 47.5% | **≥ 60%** |
| `internal/snapshot` | 29.9% | **≥ 50%** |
| `internal/virtual` | 26.0% | **≥ 40%** |

CI (the `quality-gate` job) and `make quality-gate` enforce **per-package floors** from [`quality/coverage-floors.json`](quality/coverage-floors.json). The floors start just below today's coverage and are raised toward these targets in the PRs that add the tests. The same job runs a per-function CRAP gate ([`tools/crap`](tools/crap)):
- Every Go function must score **≤ 30**.
- The 27 legacy functions above that are pinned in [`quality/crap-baseline.json`](quality/crap-baseline.json) and may not get worse.
- `make quality-baseline` lowers those ceilings as tests improve.

Additional v0.2.0 scope:
- [x] Real-database integration tests for runner SQL (Postgres, DuckDB), including injection regressions (#58)
- [ ] Reference docs: `docs/reference/sqlforge-yml.md` and `docs/reference/model-config.md`
- [ ] `sqlforge plugin install` — download pre-compiled plugin binaries for the current platform
- [ ] `docs/reference/mcp-tools.md` — full parameter/response schemas for all MCP tools

---

## 🚀 Phase 6: Ecosystem Extensibility & GitOps (Upcoming)
- [ ] **WebAssembly UDFs**: Introduce a framework allowing users to write custom User-Defined Functions (UDFs) in Go or Rust, compiling them to WASM binaries that are natively deployed into target warehouses like ClickHouse and DuckDB.
- [ ] **Native GitOps Webhooks**: Expand the `sqlforge mcp` HTTP server to securely listen to GitHub/GitLab webhooks, enabling automated DAG orchestration, zero-copy preview environments, and PR commenting directly from the SQLForge engine.
- [ ] **`sqlforge plugin install`**: Package manager command to download pre-compiled warehouse plugin binaries for the current platform.
- [ ] **BigQuery Plugin**: `sqlforge-plugin-bigquery` to complete the major cloud warehouse set.

Hosted multi-tenant SQLForge is design-only in [ADR 0008](docs/adr/0008-hosted-tenancy.md) (#63); v1 has no hosted endpoint.
