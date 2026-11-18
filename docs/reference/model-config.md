---
title: "Reference: model configuration comments"
description: Comment keys SQLForge reads from model and snapshot SQL files.
product: drover-sqlforge
audience: platform-operator
doc_type: reference
topics:
  - data-warehousing
surface: repo-docs
---

# Reference: model configuration comments

Model and snapshot settings live in the SQL file as comments. The parser keeps the SQL, including these lines, and copies each comment into a string map.

```sql
-- @key: value
```

The line must start, after trimming, with `-- @`. The key and the value are split on the first `:`. Surrounding space is trimmed. A `--` later in the value is cut off, and the value is trimmed again. A model file name must be a bare identifier: letters, digits, and underscores, not starting with a digit, at most 63 bytes. The same rule applies to schema, table, database, and column names that are interpolated into SQL.

The warehouse schema for an environment is `sqlforge__` plus the environment name. A schema fragment is appended with an underscore.

A file under `models/` with no `@schema` takes its fragment from the directory path relative to `models/`, with path separators replaced by underscores. `models/marts/marketing/fct.sql` in environment `prod` is built as `sqlforge__prod_marts_marketing.fct`. A file directly in `models/` has no fragment, so the same environment builds `sqlforge__prod.fct`.

Models loaded from an installed package set the fragment to the package name. A fragment that is already set is prefixed with `<package>_`.

## Materialisation

### `@materialized`

Default `view`.

| Value | What apply builds |
|---|---|
| `view`, or any unrecognised value | A view |
| `table` | A table, rebuilt on apply |
| `incremental` | A table on the first apply, then an incremental load |
| `materialized_view` | A materialized view |
| `kafka`, `nats`, `streaming` | A streaming table. The runner receives the value as `_materialization_type` |

### `@schema`

Schema fragment. Appended to the environment schema. See the path default above.

### `@database`

Optional database name. On apply, the object name is `database.schema.table`.

### `@alias`

Optional table name. When omitted, the table name is the model name (the file name without `.sql`).

### `@dialect`

Source dialect of the model's SQL. When it differs from the runner name and a parser is configured, apply transpiles the SQL toward the runner dialect. Empty means `ansi`.

## Incremental models

Used when `@materialized` is `incremental`.

### `@unique_key`

One column name. The load SQL interpolates it as a single identifier. With no `@incremental_strategy`, a unique key selects `merge`. Without a unique key the strategy is `append`.

### `@incremental_strategy`

| Value | Load |
|---|---|
| empty or `auto` | `merge` when `unique_key` is set, otherwise `append` |
| `append` | `INSERT INTO ... SELECT` |
| `merge` or `upsert` | Merge. Postgres and DuckDB delete matching keys, then insert, in one transaction. Snowflake, Databricks, and VeloDB use `MERGE`. Doris uses `ON DUPLICATE KEY UPDATE`. ClickHouse inserts. |
| `delete+insert` or `delete_insert` | Delete matching keys, then insert |
| `replacing_merge_tree` | ClickHouse: the first build is `ReplacingMergeTree` ordered by `unique_key`, or `tuple()` when the key is empty. Later applies insert. The engine deduplicates. |

### `@updated_at`

Version column for a ClickHouse `replacing_merge_tree`. Default column name `updated_at` when the strategy is set and this key is empty.

## Data quality

These run after the model is built, during `sqlforge apply` and `sqlforge test`. Each assertion is a count. A count above zero fails the check.

### `@test_not_null`

Comma-separated columns that must not be null.

```sql
-- @test_not_null: user_id, email
```

### `@test_unique`

Comma-separated columns that must be unique.

```sql
-- @test_unique: order_id
```

### `@test_accepted_values_<column>`

Allowed values for that column. The suffix is the column name.

```sql
-- @test_accepted_values_status: active, pending, archived
```

### `@test_relationship`

`<local_column> to <parent_model>.<parent_column>`, or the same shape with `->` instead of ` to `.

```sql
-- @test_relationship: customer_id to stg_customers.customer_id
```

The parent model is resolved inside the active environment. Nulls in the local column are not failures.

Singular tests are not model comments. A `.sql` file under `tests/` fails when it returns any row.

## Snapshots

Files under `snapshots/` use the same comment syntax. `sqlforge snapshot` and the MCP `run_snapshot` tool read these keys.

### `@strategy`

`timestamp` (default) or `check`. Any other value is an error.

### `@unique_key`

Required, unless `@grain` is set. `@grain` is the fallback for the same field.

### `@updated_at`

Required when the strategy is `timestamp`.

### `@check_cols`

Comma-separated columns. Required and non-empty when the strategy is `check`.

A snapshot with no saved state in the environment is built as a new table, even if a table of that name already exists. A later run with saved state appends or updates versions. ClickHouse timestamp and check runs insert new versions. Other dialects close the current row and insert the new one.
