# Complex Incremental Materializations Walkthrough

I have successfully designed and implemented the architecture for cross-dialect incremental materializations in the SQLForge engine! 

## What changed?

By extending the `virtual.Runner` interface, we successfully decoupled the complex logic of `MERGE/UPSERT` statements from the main topological DAG executor (`apply.go`). This allows SQLForge to dynamically generate the most efficient upsert logic based purely on the backend data warehouse target.

### 1. `TableExists` Inspection
The orchestration engine now utilizes a new `TableExists` method to peek into the target data warehouse to verify if the physical table has been instantiated yet. If it hasn't, the engine safely generates a standard `CREATE TABLE` execution on its first pass.

### 2. Dialect-Specific Upsert Generation
On subsequent runs, SQLForge will evaluate the `unique_key` property of the `incremental` model and instruct the active dialect runner to build the correct DDL:
- **DuckDB, Postgres, VeloDB**: Generates `INSERT INTO ... ON CONFLICT (...) DO UPDATE SET *`.
- **Snowflake, Databricks**: Generates standard ANSI `MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE ...`.
- **Doris**: Generates `INSERT INTO ... ON DUPLICATE KEY UPDATE *`.
- **ClickHouse**: Since ClickHouse handles merges asynchronously in the background via `ReplacingMergeTree`, the incremental run generates a highly optimized `INSERT INTO ... SELECT *` append command to respect the architecture constraints.

### 3. Example Models Updated
I updated the mock `stg_events.sql` and `daily_metrics.sql` configurations in the `agentic_retail_2026` example project to include explicit `unique_key` parameters, demonstrating the new syntax in action.

```sql
-- @materialized: incremental
-- @incremental_strategy: auto
-- @unique_key: event_id
-- @grain: event_id
```

## Validation Performed
- **Unit Tests**: Added robust string validation tests in `internal/virtual/incremental_test.go` to assert that every runner variant generates perfectly formatted SQL strings for both Append (no unique key) and Upsert (with unique key). All `TestCreateIncrementalMergeDDL` tests passed successfully.
- **E2E Testing**: Executed the `make e2e` automated pipeline. The CLI, DAG execution, and internal SQLite state trackers all ran smoothly against the modified agentic dataset models without throwing errors or panics, confirming the logic correctly cascades and intercepts the Apply commands.

The Incremental Materialization epic from the Roadmap is fully structurally integrated!
