# Contributing to SQLForge 🛠️

Welcome! We are excited to have you contribute to Drover SQLForge. As a modern, pure Go-native data modeling tool with embedded WASM intelligence, the codebase spans a few interesting paradigms. 

This guide will help you understand the architecture, set up your local environment, and write tests.

## 1. Project Structure

The repository is organized following standard Go project layouts:

- `cmd/sqlforge/`: The Cobra CLI application. This is the entrypoint.
- `internal/`: The core engine code, not meant to be imported by other Go projects.
  - `ai/`: Connections to Ollama/LLMs.
  - `config/`: YAML unmarshaling for `sqlforge.yml`.
  - `graph/`: Directed Acyclic Graph (DAG) construction and sorting algorithms.
  - `mcp/`: The Model Context Protocol (MCP) JSON-RPC HTTP server.
  - `model/`: The file loader that reads `.sql` assets and extracts `-- @config` blocks.
  - `parser/`: The **Polyglot WASM** runtime wrapper. This loads the embedded Rust parser to analyze SQL safely.
  - `plan/`: The state-diffing logic (`plan`) and the execution logic (`apply`).
  - `semantic/`: The YAML metrics engine and ANSI SQL compiler.
  - `state/`: The local SQLite persistence layer.
  - `virtual/`: The dialect-specific DDL generators (DuckDB, ClickHouse, Snowflake, etc.).
- `examples/`: Sample `sqlforge.yml` projects you can use to run the CLI locally.
- `test/e2e/`: The End-to-End CLI testing suite.

## 2. Setting Up Your Environment

You need **Go 1.23+** installed.

```bash
# Clone the repository
git clone https://github.com/drover-org/drover-sqlforge.git
cd drover-sqlforge

# Download dependencies
go mod tidy

# Build the CLI locally
make cli
```

The executable will be generated at `./sqlforge`.

## 3. The WASM Boundary

SQLForge embeds a Rust-compiled WebAssembly (WASM) module (`internal/parser/polyglot.wasm`) using the `wazero` runtime. This allows us to perform high-speed AST analysis without depending on CGO or maintaining complex regexes in Go.

Currently, the WASM blob is committed to the repository for zero-dependency local builds. If you need to make changes to the transpilation logic, you must update the Rust source (maintained in a separate repository) and copy the new `.wasm` file into `internal/parser/`.

## 4. Testing

SQLForge relies heavily on an isolated testing strategy to ensure data engineering pipelines don't break unexpectedly.

### Running Unit Tests

Run standard Go tests:
```bash
go test ./...
```

### End-to-End Tests

The E2E suite spins up the compiled CLI and runs it against the `examples/agentic_retail_2026` mock project. It verifies plan generation, apply success, semantic query compilation, and data quality failures.

```bash
make e2e
```

### Fuzz Testing

Because SQLForge parses arbitrary SQL and accepts external MCP requests, we enforce strict fuzz testing. The fuzzers feed malformed, massive, or chaotic byte slices into our core boundaries to ensure the application safely returns an `error` instead of crashing (`panic()`).

Run all Fuzz tests for 10 seconds each:
```bash
go test -fuzz=FuzzDAGBuild ./internal/graph -fuzztime=10s
go test -fuzz=FuzzParseConfigLine ./internal/model -fuzztime=10s
go test -fuzz=FuzzCompiler ./internal/semantic -fuzztime=10s
go test -fuzz=FuzzServerHTTP ./internal/mcp -fuzztime=10s
go test -fuzz=FuzzParser ./internal/parser -fuzztime=10s
```

## 5. Submitting a Pull Request

1. Ensure `make e2e` passes.
2. If adding a new feature, add a corresponding test in `test/e2e/cli_test.go` or a unit test.
3. If adding a new parser boundary or HTTP endpoint, add a Fuzz test.
4. Open a PR! The `action.yml` GitHub Action will run automatically.
