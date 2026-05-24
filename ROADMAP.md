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
- [x] **Multi-Dialect Virtual Runners**: Implement drivers for Databricks, DuckDB, Postgres, Snowflake, VeloDB, and Apache Doris to enable universal execution and localized portability.
- [x] **Deep Agentic AI Integration**: Advanced query generation and schema validation via lightweight LLMs (expanding on `sqlforge ai explain`).
- [x] **Real-Time / Streaming Engines**: Support for mapping Kafka/NATS topics to virtual envs and orchestrating ClickHouse Materialized Views.

## ✅ Phase 3: Developer Experience & Automation (Complete)
- [x] **End-to-End Testing**: Fully automated isolated execution suite (`make e2e`) validating the CLI, DAG execution, and internal state trackers against mock projects.
- [x] **Agentic Query Gen**: Agents can run `sqlforge query` to safely synthesize new queries via semantic layer primitives without seeing underlying tables.
- [x] **TUI / Web GUI**: Interactive terminal interface or lightweight local web server to visualize the DAG execution plan.
- [x] **Data Quality Testing**: Porting standard `test` macros (not_null, unique, accepted_values) into native Go verifications during `apply`.
- [x] **Incremental Materializations**: Robust cross-dialect support for complex incremental models (MERGE, UPSERT, Insert/Overwrite) utilizing AST diffing.
- [x] **CI/CD Integrations**: Pre-built GitHub Actions for spinning up isolated, zero-copy preview environments on every PR.

## ✅ Phase 4: Agentic Tier / MCP Server (Complete)
- [x] **MCP Server**: Embed an HTTP server to expose SQLForge as a Model Context Protocol service for autonomous agents.
- [x] **MCP Tool Registry**: Read and mutation tools (`list_models`, `query_metric`, `plan_change`, `apply_change`, etc.); ephemeral plan store for `apply_change` ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).
- [x] **Interactive Execution**: WebSocket endpoints for live plan approval and step-by-step execution feedback for agents.
- [x] **Security & Observability**: API key auth, rate limiting, and comprehensive request auditing for all agent interactions.

## ✅ Phase 5.1: Lineage & env e2e (Complete)
- [x] **Column lineage CLI**: `sqlforge lineage [model]` with structural SELECT/FROM parsing.
- [x] **MCP column_lineage**: `get_model` returns per-column upstream refs.
- [x] **env create e2e**: `TestE2EEnvCreate` validates CLI + SQLite state for preview environments.

## ✅ Phase 5: Stability & Developer Onboarding (Complete)
- [x] **End-to-End Testing Expansion**: Full coverage for incremental workflows, data quality assertions, and CLI integrations.
- [x] **Fuzz Testing Security**: High-throughput property testing on the WASM parser boundary and JSON-RPC MCP server.
- [x] **Onboarding Material**: Comprehensive `CONTRIBUTING.md` created to outline architecture, local environment setup, and compilation.
- [x] **Code Stubs Resolution**: Code stubs evaluated, mocks fixed, and known limitations documented.

## 🚀 Phase 6: Ecosystem Extensibility & GitOps (Upcoming)
- [ ] **WASM AST Transpilation**: Leverage the polyglot WASM parser to automatically transpile SQL dialects on the fly (e.g., compile Snowflake syntax down to Postgres/DuckDB for local development) without altering the source `.sql` files.
- [ ] **WebAssembly UDFs**: Introduce a framework allowing users to write custom User-Defined Functions (UDFs) in Go or Rust, compiling them to WASM binaries that are natively deployed into target warehouses like ClickHouse and DuckDB.
- [ ] **Native GitOps Webhooks**: Expand the `sqlforge mcp` HTTP server to securely listen to GitHub/GitLab webhooks, enabling automated DAG orchestration, zero-copy preview environments, and PR commenting directly from the SQLForge engine.
