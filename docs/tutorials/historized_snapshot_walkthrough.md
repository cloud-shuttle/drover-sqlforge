---
title: Historized snapshot walkthrough
description: Tutorial for SCD Type 2 historized snapshot models in SQLForge.
product: drover-sqlforge
audience:
  - platform-operator
  - agent
doc_type: tutorial
topics:
  - data-warehousing
surface: repo-docs
---

# Historized snapshot walkthrough

> **Terms:** [`CONTEXT.md`](../../CONTEXT.md) — **Historized snapshot**, **Snapshot definition**. **ADR:** [0004](../adr/0004-historized-snapshot.md).

## Layout

```
snapshots/
  users_snapshot.sql
```

Example [`examples/agentic_retail_2026/snapshots/users_snapshot.sql`](../../examples/agentic_retail_2026/snapshots/users_snapshot.sql):

```sql
-- @strategy: timestamp
-- @unique_key: user_id
-- @updated_at: created_at

SELECT ...
```

## Run

```bash
cd examples/agentic_retail_2026
../../sqlforge snapshot prod
```

First run performs an **initial build** (creates `sqlforge__prod.users_snapshot` with `sqlforge_valid_from` / `sqlforge_valid_to`). Later runs perform a timestamp **incremental run** (close changed current rows, insert new versions).

## Strategies

| Strategy | v1 status |
|----------|-----------|
| `timestamp` | Supported (DuckDB/Postgres/Snowflake-style SQL; ClickHouse append-only) |
| `check` | Deferred |

## CI

Run after models are applied in a **preview environment**, or against `prod` in controlled pipelines. Same CLI path as local **data engineers** ([ADR 0002](../adr/0002-cli-invocation-drover-code-integration.md)).
