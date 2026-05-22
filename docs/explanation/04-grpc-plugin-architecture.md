---
title: gRPC plugin architecture for virtual runners
description: Design for migrating SQLForge virtual runners to a gRPC plugin model.
product: drover-sqlforge
audience: platform-operator
doc_type: explanation
topics:
  - data-warehousing
  - deployment
surface: repo-docs
---

# Migrating Virtual Runners to a gRPC Plugin Architecture

As SQLForge approaches v1.0 and expands its footprint to support numerous dialects (Snowflake, BigQuery, Postgres, DuckDB, Databricks), embedding all dialect-specific drivers directly into the core `sqlforge` binary via `go.mod` is an unsustainable anti-pattern. 

It leads to:
1. **Massive Binary Bloat:** Every driver adds megabytes to the binary.
2. **CGO Nightmares:** Drivers like `go-duckdb` rely on C-bindings, breaking simple cross-compilation and demanding local C++ compilers.
3. **Dependency Hell:** Conflicting module versions between distinct drivers.

To cleanly support a massive ecosystem of dialects, SQLForge will migrate the `virtual.Runner` interface to a **gRPC Plugin Architecture** using HashiCorp's `go-plugin`.

## The Architecture

Under the new model, the core SQLForge executable becomes a lightweight orchestrator. It manages the CLI, state, Polyglot WASM parser, and DAG generation. It completely drops `database/sql` dependencies.

When a user specifies `dialect: snowflake` in their `sqlforge.yml`, the engine will:
1. Look for an executable named `sqlforge-runner-snowflake` in the system path or `.sqlforge/plugins/`.
2. Spawn the executable as a background process.
3. Establish a gRPC connection over a local unix socket.
4. Issue DDL commands and validation queries over the network boundary.

### The Protobuf Interface

The current Go interface `virtual.Runner` will be converted into a `Runner` Protobuf service:

```protobuf
syntax = "proto3";
package runner;

service RunnerPlugin {
  rpc Init(InitRequest) returns (InitResponse);
  
  // Execution
  rpc Exec(ExecRequest) returns (ExecResponse);
  rpc QueryCount(QueryCountRequest) returns (QueryCountResponse);
  
  // DDL Generation
  rpc CreateSchemaDDL(CreateSchemaRequest) returns (DDLResponse);
  rpc CreateTableDDL(CreateTableRequest) returns (DDLResponse);
  rpc CreateViewDDL(CreateViewRequest) returns (DDLResponse);
  rpc CreateIncrementalMergeDDL(CreateMergeRequest) returns (DDLResponse);
}
```

### Implementing a Dialect Plugin

To add support for a new database (e.g., DuckDB), a contributor will create a *separate GitHub repository* (e.g., `drover-org/sqlforge-runner-duckdb`).

This repository will import `github.com/hashicorp/go-plugin` and serve the gRPC endpoints. Because it is an isolated binary, it can safely import `github.com/marcboeker/go-duckdb` and compile with CGO without poisoning the core SQLForge project. 

```go
// Example Plugin Entrypoint
func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: shared.Handshake,
        Plugins: map[string]plugin.Plugin{
            "runner": &shared.RunnerGRPCPlugin{Impl: &DuckDBRunner{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

### Distribution

Instead of shipping a monolithic binary, SQLForge will adopt a package-manager approach. Users will run:
```bash
sqlforge plugin install snowflake
```
Which downloads the compiled `sqlforge-runner-snowflake` binary for their architecture.

## Roadmap for v1.0
1. Define the `.proto` schema in `internal/plugin/proto`.
2. Implement the `go-plugin` client in `internal/virtual/manager.go`.
3. Extract `ClickHouseRunner` into its own repository: `sqlforge-runner-clickhouse`.
4. Remove `database/sql` from the core project.
