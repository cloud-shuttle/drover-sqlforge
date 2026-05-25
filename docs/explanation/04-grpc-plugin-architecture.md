---
title: gRPC plugin architecture for virtual runners
description: How SQLForge warehouse drivers work as standalone gRPC plugin binaries.
product: drover-sqlforge
audience: platform-operator
doc_type: explanation
topics:
  - data-warehousing
  - deployment
surface: repo-docs
---

# gRPC Plugin Architecture for Virtual Runners

The core `sqlforge` binary is **CGO-free and has no warehouse drivers**. All database connectivity is handled by standalone **gRPC plugin binaries** (`sqlforge-plugin-<dialect>`) that are spawned on demand via [HashiCorp `go-plugin`](https://github.com/hashicorp/go-plugin).

This design solves three fundamental problems with the alternative monolithic approach:

1. **Binary bloat** — every driver adds megabytes. DuckDB's CGO library alone is ~60 MB.
2. **CGO hell** — drivers like `go-duckdb` require a local C++ compiler and break simple cross-compilation.
3. **Dependency conflicts** — distinct driver packages routinely pin incompatible transitive dependencies.

---

## How It Works

When `sqlforge plan prod` (or `apply`, `test`, etc.) is invoked:

```
sqlforge plan prod
  └─ internal/virtual/runner_factory.go
      └─ loadRunnerPlugin("clickhouse", dsn)
          ├─ resolves binary: sqlforge-plugin-clickhouse (PATH or same dir)
          ├─ spawns subprocess (os/exec)
          ├─ establishes gRPC over unix socket
          └─ returns virtual.Runner — identical interface to any in-process runner
```

The factory (`internal/virtual/runner_factory.go`) uses `SQLFORGE_PLUGIN_DSN` to pass the warehouse connection string to the child process. All DDL generation, query execution, and schema introspection flows over the gRPC boundary.

---

## Plugin Binary Map

| Dialect | Binary | Driver | Notes |
|---------|--------|--------|-------|
| `clickhouse` | built-in | `clickhouse-go/v2` | In-process (no CGO) |
| `duckdb` | `sqlforge-plugin-duckdb` | `go-duckdb` (CGO) | CGO isolated in plugin |
| `snowflake` | `sqlforge-plugin-snowflake` | `gosnowflake` | — |
| `databricks` | `sqlforge-plugin-databricks` | `databricks-sql-go` | — |
| `postgres` | `sqlforge-plugin-postgres` | `lib/pq` | — |
| `doris` | `sqlforge-plugin-doris` | `go-sql-driver/mysql` | MySQL wire protocol |
| `velodb` | `sqlforge-plugin-velodb` | `go-sql-driver/mysql` | MySQL wire protocol (SelectDB fork) |

---

## The `virtual.Runner` Interface

All plugins implement the same Go interface:

```go
type Runner interface {
    Name() string
    Exec(ctx context.Context, query string) error
    QueryCount(ctx context.Context, sql string) (int, error)
    QueryData(ctx context.Context, sql string) ([]map[string]interface{}, error)
    TableExists(ctx context.Context, schema, table string) (bool, error)

    // DDL generators — return dialect-specific SQL strings
    CreateSchemaDDL(schema string) string
    CreateTableDDL(schema, table, selectSQL string) string
    CreateViewDDL(schema, table, selectSQL string) string
    CreateMaterializedViewDDL(schema, table, selectSQL string) string
    CreateStreamingTableDDL(schema, table string, config map[string]string) string
    CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string
}
```

The protobuf service and gRPC client/server wrappers live in `internal/virtual/proto/` and `internal/virtual/runner_grpc.go`.

---

## Writing a New Plugin

To add support for a new warehouse (e.g., BigQuery):

1. Create `cmd/plugins/sqlforge-plugin-bigquery/main.go`.
2. Implement all methods of `virtual.Runner` using the appropriate Go driver.
3. Call `plugin.Serve` with `virtual.Handshake` and `virtual.RunnerGRPCPlugin`:

```go
func main() {
    dsn := os.Getenv("SQLFORGE_PLUGIN_DSN")

    // Connect using your driver
    db, err := sql.Open("bigquery", dsn)
    ...

    runner := &BigQueryRunner{db: db}

    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: virtual.Handshake,
        Plugins: map[string]plugin.Plugin{
            "runner": &virtual.RunnerGRPCPlugin{Impl: runner},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

4. Add a build rule to the `Makefile` `plugins` target.
5. Implement DDL helpers using `virtual.BuildIncrementalMergeDDL` for incremental strategies.

Because it is an isolated binary, your plugin can freely import CGO-based or platform-specific drivers without affecting the core `sqlforge` binary.

---

## Building All Plugins

```bash
make plugins
```

This compiles all plugin binaries to the repo root so they are co-located with the `sqlforge` binary:

```
./sqlforge
./sqlforge-plugin-duckdb
./sqlforge-plugin-snowflake
./sqlforge-plugin-databricks
./sqlforge-plugin-doris
./sqlforge-plugin-velodb
```

---

## Future: `sqlforge plugin install`

Phase 6 will introduce a package manager sub-command that downloads pre-compiled plugin binaries for the current platform from GitHub Releases:

```bash
sqlforge plugin install bigquery
```
