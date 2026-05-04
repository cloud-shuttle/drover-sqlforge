# Implementation Plan: Complex Incremental Materialization

## Goal Description
Currently, SQLForge treats `materialized: incremental` models identically to `table` models, completely overwriting them upon every execution. Our objective is to design and implement robust, cross-dialect incremental logic (Append, Upsert/Merge, Insert-Overwrite) so that massive datasets can be updated efficiently without full rebuilds.

## User Review Required
> [!IMPORTANT]
> **ClickHouse Limitations on MERGE:** Standard ClickHouse `MergeTree` does not natively support row-level `UPSERT` or `MERGE INTO` in the way Postgres or Snowflake do. We must choose a default strategy for ClickHouse. 
> I propose that for ClickHouse, we default to **Append-Only** (INSERT INTO) unless the model config explicitly specifies `engine: ReplacingMergeTree`, at which point the runner will use that engine type during initial creation to allow ClickHouse to handle the "upserts" natively in the background. Does this approach align with your expectations for the ClickHouse runner?

## Open Questions
> [!NOTE]
> 1. Should we support a `delete+insert` strategy where we dynamically inject a `DELETE FROM target WHERE time >= (SELECT MIN(time) FROM source)` before inserting?
> 2. How should we handle schema drift on incremental models? Should SQLForge automatically detect column additions and run `ALTER TABLE ... ADD COLUMN` before merging, or should it fail and require the user to run a `--full-refresh`?

## Proposed Architecture

To abstract the complex dialect-specific behaviors away from the core `ApplyPlan` engine, we will extend the `virtual.Runner` interface. 

### Core Changes

#### [MODIFY] internal/virtual/runner.go
Add the following methods to the interface:
```go
// Checks if the physical target table already exists in the environment
TableExists(ctx context.Context, schema, table string) (bool, error)

// Generates the specific dialect's DDL to merge/append new data
CreateIncrementalMergeDDL(schema, table, selectSQL string, config map[string]string) string
```

#### [MODIFY] internal/plan/apply.go
Update the `ApplyPlan` loop to utilize the new incremental logic:
```go
if mat == "incremental" {
    exists, err := vMgr.Runner().TableExists(ctx, schema, a.Name)
    if err != nil { return err }
    
    if !exists {
        // Initial run behaves like a table build
        ddl = vMgr.Runner().CreateTableDDL(schema, a.Name, transpiledSQL)
    } else {
        // Subsequent runs use the complex merge logic
        ddl = vMgr.Runner().CreateIncrementalMergeDDL(schema, a.Name, transpiledSQL, a.Config)
    }
}
```

---

## Dialect-Specific Implementations

### [MODIFY] internal/virtual/runner_duckdb.go
DuckDB natively supports `INSERT INTO ... ON CONFLICT DO UPDATE`.
- If `unique_key` is provided in the model `config`: We will generate an `INSERT INTO target SELECT * FROM (source) ON CONFLICT (unique_key) DO UPDATE SET ...`
- If no `unique_key` is provided: We will default to a standard `INSERT INTO target SELECT * FROM (source)`.

### [MODIFY] internal/virtual/runner_snowflake.go
Snowflake supports standard ANSI `MERGE INTO`.
- We will parse the `unique_key` and generate a `MERGE INTO target USING (source) ON target.id = source.id WHEN MATCHED THEN UPDATE ... WHEN NOT MATCHED THEN INSERT ...`

### [MODIFY] internal/virtual/runner_postgres.go
Postgres supports `ON CONFLICT (...) DO UPDATE SET ...` similar to DuckDB. The runner will generate this specific syntax.

### [MODIFY] internal/virtual/runner_clickhouse.go
Since ClickHouse is an append-optimized OLAP database:
- `CreateIncrementalMergeDDL` will default to `INSERT INTO target SELECT * FROM (source)`.
- If a user needs upsert behavior, they will define `incremental_strategy: replacing_merge_tree` in the model config, and `CreateTableDDL` will create the table with `ENGINE = ReplacingMergeTree(updated_at)`. The incremental merge DDL remains a simple `INSERT`, and ClickHouse handles the deduplication asynchronously.

---

## Verification Plan

### Automated Tests
- Create `test/incremental_test.go` to validate DDL generation across all 4 runner variants (DuckDB, ClickHouse, Postgres, Snowflake).
- Add a property-based test to ensure all combinations of `unique_key` and `incremental_strategy` do not result in malformed SQL.

### E2E Testing
- Update `examples/agentic_retail_2026/models/stg_events.sql` to include an `incremental_strategy: append` and run the `make e2e` pipeline twice to assert that `TableExists` is hit and `INSERT INTO` is utilized on the second pass instead of `CREATE TABLE`.
