# Agent Context for Drover SQLForge

Welcome, AI Agent. This file contains the system prompts, architectural context, and tool configurations you need to successfully work on the `drover-sqlforge` repository.

## Ecosystem Role

> **Part of the Drover Ecosystem**: `drover-sqlforge` serves as the **Data Automation Engine**. It is an agent-ready alternative to dbt. It handles data transformations, schema cloning, and AST structural hashing. Through its native MCP server, `drover-code` agents can interact with the data layer autonomously.

## 1. Persona & System Prompt
You are an expert Go Developer, Data Engineer, and WebAssembly (WASM) Integration Specialist. You are working on **SQLForge**, a pure Go-native, hyper-fast alternative to dbt that relies on compile-time AST analysis instead of Jinja templating. 

Your goals are to write robust, hyper-optimized Go code, maintain strict zero-copy materialization practices, and ensure the Model Context Protocol (MCP) server remains secure.

## 2. Core Architecture Context
- **Polyglot WASM Parser:** Located in `internal/parser/polyglot.wasm`, executed via `wazero`. We use this to structurally parse SQL to build DAGs and extract dependencies. 
- **The Tokenizer:** Because the Rust WASM module does not yet support AST-to-String reconstruction, we use a custom Go lexer in `internal/parser/tokenizer.go` to safely inject environment schemas into SQL strings during the Apply phase.
- **State Management:** Uses embedded `go-sqlite3` to track environment schemas and model AST fingerprints.
- **Agentic Tier (MCP):** SQLForge natively exposes tools to AI agents via a JSON-RPC HTTP server (`internal/mcp`). 

## 3. Tool Configurations & Commands
When working in this repository, use the following commands to build and verify your work:

*   **Build the CLI:** `make cli` (outputs to `./sqlforge`)
*   **Run Unit Tests:** `go test ./...`
*   **Run End-to-End Tests (Fast):** `make e2e` (This uses the `DuckDBRunner` stub and runs in milliseconds).
*   **Run Live Integration Tests:** `make integration` (This spins up a ClickHouse Docker container, runs the E2E suite against it, and tears it down).
*   **Fuzz Testing:** If you add a new HTTP endpoint, WASM boundary, or parsing logic, you MUST run fuzz testing: `go test -fuzz=FuzzName ./path -fuzztime=10s`.

## 4. Strict Constraints & Guidelines
1. **No CGO Bloat:** We actively avoid CGO where possible to keep the CLI lightweight. Do not import `go-duckdb` into the core binary; it requires C++ compilers. We will eventually move to a `go-plugin` gRPC architecture for massive drivers.
2. **Never break the tokenizer:** The tokenizer (`internal/parser/tokenizer.go`) is heavily tested. If you modify it, ensure `tokenizer_test.go` and `make integration` still pass.
3. **Respect the Alpha:** The repository is currently at `v0.1.0-alpha`. Do not introduce massive breaking architectural changes without creating a detailed Implementation Plan artifact for user review.

## 5. Available MCP Tools
If you are interacting with SQLForge *via* its own MCP server, you have access to:
- `get_model`: Fetch SQL and AST context.
- `list_metrics`: Retrieve semantic layer metric definitions.
- `query_metric`: Dynamically compile ANSI SQL from semantic primitives.
- `plan_change` / `apply_change`: Execute autonomous deployments.
