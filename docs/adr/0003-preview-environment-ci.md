---
title: "ADR 0003: Preview environment CI pattern"
description: Zero-copy preview environments in CI for safe warehouse change validation.
product: drover-sqlforge
audience: platform-operator
doc_type: adr
topics:
  - data-warehousing
  - deployment
surface: repo-docs
---

# ADR 0003: Preview environment CI pattern

**Status:** Accepted  
**Date:** 2026-05-22  
**Contexts:** [`CONTEXT.md`](../../CONTEXT.md)

## Context

Pull requests that change **models** or **metrics** need isolated validation on real warehouse data without writing to `prod`. SQLForge supports **environments** (named plan/apply targets), **zero-copy isolation** on ClickHouse, and a composite GitHub Action (`action.yml`) that:

1. Creates a **preview environment** named `pr_{number}` with **base environment** `prod` (configurable).
2. Runs `sqlforge plan` and comments output on the PR.
3. Runs `sqlforge apply` with `CI=true` (non-TTY).

Alternatives considered:

1. **Plan-only CI** — post **execution plan** on PR; **apply** waits for human approval in console or separate workflow.
2. **Shared staging env per branch** — reuse `staging` instead of per-PR env names; risk cross-PR contamination.
3. **Physical full copy** — clone prod tables without zero-copy; higher cost and time.
4. **No CI integration** — manual `sqlforge plan` locally only.

## Decision

### 1. Preview environment naming

- **Preview environment:** `pr_{pull_request_number}` (or explicit `environment` input).
- **Base environment (CI):** defaults to `prod` — the lineage parent for **warehouse schema** creation and **zero-copy isolation** where supported.
- Each PR gets an isolated **environment** and **warehouse schema** (`sqlforge__pr_*`), not shared staging unless configured.

### 2. v1 GitHub Action behaviour

The composite action in [`action.yml`](../../action.yml) runs **plan** then **apply** automatically:

| Step | Purpose |
|------|---------|
| `sqlforge plan` | Produce **execution plan**; comment on PR when `github_token` provided |
| `sqlforge apply` | Materialize **changed** and **impacted models** into the **preview environment** |

`CI=true` disables interactive TUI during **apply**.

`config_dir` defaults to `.` (**project root** relative to checkout).

### 3. CI apply gate (deferred)

**CI apply gate** — policy where CI posts **plan** only and **apply** requires a follow-up approval — is **not** the v1 default. Teams that need it can:

- split workflows (plan job + manual `workflow_dispatch` apply), or
- add a future `apply: false` input on the composite action.

Document as **deferred** in glossary; do not block current action on gate UX.

### 4. Credentials and project layout

- Warehouse credentials come from the CI secret store / `.env` expansion in `sqlforge.yml` — same as local **data engineers**.
- **Project state** (`.sqlforge/state.db`) is ephemeral in CI runners; **preview environment** state is recreated per run unless cached by the platform.

## Consequences

**Positive**

- PR authors and reviewers see structural diffs before merge; isolated data plane reduces prod risk.
- Aligns with **zero-copy isolation** differentiator on ClickHouse.
- Reuses standard **plan** / **apply** vocabulary from [`CONTEXT.md`](../../CONTEXT.md).

**Negative**

- Automatic **apply** on every PR sync may be too aggressive for large warehouses or costly models—teams may need plan-only forks until **CI apply gate** ships.
- Non-ClickHouse **warehouse connections** may not get true zero-copy; preview envs still work but may be slower/more expensive.
- Orphan **preview environments** (`pr_*`) need operational cleanup policies outside SQLForge v1.

**Follow-ups**

- Add `apply: false` (or `plan_only: true`) input to `action.yml`.
- Document teardown job (`env destroy` or warehouse DDL) for stale `pr_*` schemas.
- Link from [`examples/github-actions/pr-preview.yml`](../../examples/github-actions/pr-preview.yml) to this ADR.

## References

- Glossary: [`CONTEXT.md`](../../CONTEXT.md) — § CI and preview environments, § Environments
- Composite action: [`action.yml`](../../action.yml)
- Example workflow: [`examples/github-actions/pr-preview.yml`](../../examples/github-actions/pr-preview.yml)
