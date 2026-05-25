# SQLForge Roadmap

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

## ✅ Phase 4: Agentic Tier / MCP Server (Complete)
- [x] **MCP Server**: Embed an HTTP server to expose SQLForge as a Model Context Protocol service for autonomous agents.
- [x] **MCP Tool Registry**: Read and mutation tools (`list_models`, `query_metric`, `plan_change`, `apply_change`, etc.); ephemeral plan store for `apply_change` ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).
- [x] **Interactive Execution**: WebSocket endpoints for live plan approval and step-by-step execution feedback for agents.
- [x] **Security & Observability**: API key auth, rate limiting, and comprehensive request auditing for all agent interactions.

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
| `internal/plan` | 66.7% | **≥ 80%** |
| `internal/state` | 47.5% | **≥ 60%** |
| `internal/snapshot` | 29.9% | **≥ 50%** |
| `internal/virtual` | 26.0% | **≥ 40%** |

Coverage targets are enforced by the quality gate (`make quality-gate` / CI `quality-gate` job).

Additional v0.2.0 scope:
- [ ] Reference docs: `docs/reference/sqlforge-yml.md` and `docs/reference/model-config.md`
- [ ] `sqlforge plugin install` — download pre-compiled plugin binaries for the current platform
- [ ] `docs/reference/mcp-tools.md` — full parameter/response schemas for all MCP tools

---

## 🚀 Phase 6: Ecosystem Extensibility & GitOps (Upcoming)
- [ ] **WebAssembly UDFs**: Introduce a framework allowing users to write custom User-Defined Functions (UDFs) in Go or Rust, compiling them to WASM binaries that are natively deployed into target warehouses like ClickHouse and DuckDB.
- [ ] **Native GitOps Webhooks**: Expand the `sqlforge mcp` HTTP server to securely listen to GitHub/GitLab webhooks, enabling automated DAG orchestration, zero-copy preview environments, and PR commenting directly from the SQLForge engine.
- [ ] **`sqlforge plugin install`**: Package manager command to download pre-compiled warehouse plugin binaries for the current platform.
- [ ] **BigQuery Plugin**: `sqlforge-plugin-bigquery` to complete the major cloud warehouse set.
