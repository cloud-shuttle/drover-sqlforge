# SQLForge Roadmap

## ✅ Phase 1: Foundation & MVP Engine (Complete)
- [x] **Polyglot WASM Parser Integration**: Cross-platform, sub-millisecond AST generation for model extraction.
- [x] **Model Loader & DAG Builder**: Structural reference resolution and chronological dependency tracking.
- [x] **Zero-Copy Virtual Environments**: Idempotent materializations utilizing ClickHouse `CLONE` and `VIEW`.
- [x] **State Management**: Embedded SQLite to track model AST fingerprints and environment schemas.
- [x] **Plan / Apply Pipeline**: Topological sorting and execution logic for live cloud deployments.
- [x] **Self-Contained Examples**: The `agentic_retail_2026` mock dataset running seamlessly via ClickHouse generator functions.

## 🚀 Phase 2: Advanced Data Infrastructure (In Progress)
- [x] **Native Semantic Layer**: Metric parsing (`metrics.yml`) and dialect-agnostic ANSI SQL compiler.
- [x] **Hybrid Materialization**: Dynamic injection of semantic metrics into the execution DAG (`materialize: true`).
- [x] **Property-Based Testing**: Fuzzer validation for the semantic compiler to prevent SQL injection or panics.
- [ ] **Multi-Dialect Virtual Runners**: Implement DuckDB, Postgres, and Snowflake drivers for the `Runner` interface to enable localized execution and cloud portability.
- [ ] **Deep Agentic AI Integration**: Advanced query generation and schema validation via lightweight LLMs (expanding on `sqlforge ai explain`).
- [ ] **Real-Time / Streaming Engines**: Support for mapping Kafka topics to virtual envs and orchestrating ClickHouse Materialized Views.

## 🔮 Phase 3: Developer Experience & Automation (Planned)
- [ ] **TUI / Web GUI**: Interactive terminal interface or lightweight local web server to visualize the DAG execution plan.
- [ ] **Data Quality Testing**: Porting standard `test` macros (not_null, unique, accepted_values) into native Go verifications during `apply`.
- [ ] **CI/CD Integrations**: Pre-built GitHub Actions for spinning up isolated, zero-copy preview environments on every PR.
