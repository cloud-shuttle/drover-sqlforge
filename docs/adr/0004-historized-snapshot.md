---
title: "ADR 0004: Historized snapshots (SCD Type 2)"
description: Decision record for historized snapshot materialization in SQLForge.
product: drover-sqlforge
audience: platform-operator
doc_type: adr
topics:
  - data-warehousing
surface: repo-docs
---

# ADR 0004: Historized snapshots (SCD Type 2)

**Status:** Accepted  
**Date:** 2026-05-22  
**Contexts:** [`CONTEXT.md`](../../CONTEXT.md)

## Context

Slowly changing dimensions (SCD Type 2) require historized rows: when a source record changes, close the current version (`valid_to`) and insert a new current version (`valid_from`). dbt provides `dbt snapshot` with `timestamp` and `check` strategies. SQLForge listed **historized snapshot** as a **deferred capability** in [`GAP_ANALYSIS.md`](../explanation/GAP_ANALYSIS.md)—a common migration blocker from dbt.

Alternatives considered:

1. **Manual SCD SQL in models** — data engineers write `MERGE` boilerplate; no standard command.
2. **Snapshot as `materialized: snapshot` on models** — mixes transformation DAG with historization; harder to plan separately.
3. **Dedicated `snapshots/` + `sqlforge snapshot`** — mirrors dbt layout; pure SQL + `-- @` config (no Jinja).
4. **Type 1 only (overwrite)** — insufficient for audit/history use cases.

## Decision

### 1. Layout and config

- Snapshot definitions live under `snapshots/*.sql` (pure SQL + `-- @` **model config** lines).
- CLI: `sqlforge snapshot [environment] [snapshot_name...]` — runs all snapshots if names omitted.
- Snapshots are **not** part of the **model DAG**; they run on demand (like dbt), not via `plan`/`apply`.

Required config:

| Key | Required | Purpose |
|-----|----------|---------|
| `strategy` | No (default `timestamp`) | `timestamp` in v1; `check` deferred |
| `unique_key` | Yes* | Business key column(s), comma-separated |
| `updated_at` | Yes for `timestamp` | Column detecting row changes |

\*If `unique_key` is omitted, fall back to `grain` when present.

### 2. SCD columns

SQLForge injects two historization columns on the snapshot table:

- `sqlforge_valid_from` — when the row version became current
- `sqlforge_valid_to` — when superseded; `NULL` for the current version

Names are prefixed to avoid colliding with source columns.

### 3. Timestamp strategy (v1)

**Initial build** (snapshot table missing):

- `CREATE TABLE … AS SELECT <source>, sqlforge_valid_from, sqlforge_valid_to FROM (<source SQL>)`

**Incremental run**:

1. Stage source into a temp relation
2. `UPDATE` current rows (`sqlforge_valid_to IS NULL`) where `updated_at` increased
3. `INSERT` new current rows for new keys and changed keys

Implemented for runners using ANSI-style `UPDATE … FROM` / `INSERT` (DuckDB stub, Postgres, Snowflake, Databricks, VeloDB, Doris stubs). **ClickHouse** uses append-only `INSERT` in v1 (full row versions); operators may later use `ReplacingMergeTree` engines—documented limitation.

**Check strategy:** deferred (returns a clear error).

### 4. State

`snapshot_states` in `.sqlforge/state.db` stores per-**environment** **fingerprint** (source SQL + config) and `last_applied`—for future plan integration; v1 always executes on `sqlforge snapshot`.

### 5. Agents

**Agents** and CI use the same CLI path as **data engineers** ([ADR 0002](0002-cli-invocation-drover-code-integration.md)). MCP snapshot tools are **deferred**.

## Consequences

**Positive**

- Closes the largest dbt parity gap without Jinja.
- Reuses `-- @` config and **project root** conventions.
- **Grain** / **unique key** glossary terms align with snapshot config.

**Negative**

- ClickHouse historization is append-only in v1—not row-level `UPDATE`.
- No `sqlforge plan` integration for snapshots yet.
- **Schema drift** on snapshot tables follows global policy (fail/manual **full refresh** deferred).

**Follow-ups**

- `check` strategy (column hash compare).
- `sqlforge snapshot plan` and fingerprint-only skip.
- ClickHouse `ReplacingMergeTree` + version column option.
- MCP `list_snapshots` / `run_snapshot` for **agents**.

## References

- Glossary: [`CONTEXT.md`](../../CONTEXT.md) — **Historized snapshot**
- Gap analysis: [`GAP_ANALYSIS.md`](../explanation/GAP_ANALYSIS.md)
- ADR 0001: No-Jinja policy
