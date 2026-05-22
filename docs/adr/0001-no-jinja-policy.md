---
title: "ADR 0001: No-Jinja policy for SQL models"
description: Pure SQL models with compile-time AST dependency resolution instead of Jinja templating.
product: drover-sqlforge
audience: platform-operator
doc_type: adr
topics:
  - data-warehousing
surface: repo-docs
---

# ADR 0001: No-Jinja policy for SQL models

**Status:** Accepted  
**Date:** 2026-05-22  
**Contexts:** [`CONTEXT.md`](../../CONTEXT.md)

## Context

dbt and SQLMesh rely on Jinja templating (`{{ ref() }}`, macros, `for` loops) to generate SQL at runtime. Teams migrating to **Drover SQLForge** expect macro packages and dynamic column generation. SQLForge instead parses **pure SQL models** with a compile-time WASM AST and resolves **structural references** without a preprocessor.

Alternatives considered:

1. **Optional Jinja layer** — Jinja pre-processes SQL before AST parsing; preserves dbt package compatibility.
2. **dbt-compatible `ref()` syntax only** — no macros, but special-case `{{ ref('model') }}` in strings.
3. **Pure SQL only (no templating)** — dependencies from AST/table analysis; reusable logic in warehouse UDFs or shared SQL packages.
4. **Python models instead of Jinja** — PySpark/Python transforms for non-SQL logic (see gap analysis).

## Decision

Adopt a permanent **no-Jinja policy**:

- **Models** are warehouse-runnable SQL files. No runtime string templating, macro expansion, or `{{ ref() }}`.
- **Model dependencies** are **structural references** extracted by the Polyglot WASM parser (`ExtractRefs`), not macro output.
- Reusable logic belongs in:
  - warehouse UDFs and native SQL,
  - future **SQL package import** (Git repos of pure SQL + `metrics.yml`, not macro packages),
  - upstream **batch models** in the **model DAG**.
- **Python models** remain a **non-goal** (single-binary, pure-SQL vision).

This is a product constraint, not a missing feature. Documented in [`CONTEXT.md`](../../CONTEXT.md) § Principles.

## Consequences

**Positive**

- IDE-compatible, copy-pasteable SQL; no “spaghetti” macro indirection.
- **Fingerprints** and **plan** diffs reflect logical SQL structure, not rendered Jinja output.
- Clear positioning vs dbt/SQLMesh for agent-native, compile-time tooling.

**Negative**

- No drop-in dbt package hub (Fivetran-style macros); migration requires rewriting macros as SQL/UDFs.
- Teams with heavy Jinja codegen need a mindset shift; sales conversations must set expectations early.
- **Agents** cannot “add a macro”; they edit **models** and **metrics** only.

**Follow-ups**

- Implement **SQL package import** ([`GAP_ANALYSIS.md`](../explanation/GAP_ANALYSIS.md)) without Jinja.
- Keep README/agents.md aligned with glossary terms (**pure SQL model**, **structural reference**).

## References

- Glossary: [`CONTEXT.md`](../../CONTEXT.md) — § Principles, § Models
- Gap analysis: [`GAP_ANALYSIS.md`](../explanation/GAP_ANALYSIS.md) — intentional omissions
