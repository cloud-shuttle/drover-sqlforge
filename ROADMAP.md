# SQLForge Roadmap

## ✅ Phase 1: Foundation & MVP Engine (Complete)
- [x] **Polyglot WASM Parser Integration**: Cross-platform, sub-millisecond AST generation for model extraction.
- [x] **Model Loader & DAG Builder**: Structural reference resolution and chronological dependency tracking.
- [x] **Zero-Copy Virtual Environments**: Idempotent materializations utilizing ClickHouse `CLONE` and `VIEW`.
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

## 🚀 Phase 3: Developer Experience & Automation (In Progress)
- [x] **End-to-End Testing**: Fully automated isolated execution suite (`make e2e`) validating the CLI, DAG execution, and internal state trackers against mock projects.
- [x] **TUI / Web GUI**: Interactive terminal interface or lightweight local web server to visualize the DAG execution plan.
- [x] **Data Quality Testing**: Porting standard `test` macros (not_null, unique, accepted_values) into native Go verifications during `apply`.
- [x] **Incremental Materializations**: Robust cross-dialect support for complex incremental models (MERGE, UPSERT, Insert/Overwrite) utilizing AST diffing.
- [ ] **CI/CD Integrations**: Pre-built GitHub Actions for spinning up isolated, zero-copy preview environments on every PR.

## ✅ Phase 4: Agentic Tier / MCP Server (Complete)
- [x] **MCP Server**: Embed an HTTP server to expose SQLForge as a Model Context Protocol service for autonomous agents.
- [x] **MCP Tool Registry**: Expose tools like `list_models`, `query_metric`, `plan_change`, and `apply_change` for agent usage.
- [x] **Interactive Execution**: WebSocket endpoints for live plan approval and step-by-step execution feedback for agents.
- [x] **Security & Observability**: API key auth, rate limiting, and comprehensive request auditing for all agent interactions.
