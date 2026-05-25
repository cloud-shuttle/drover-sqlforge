# Drover SQLForge — domain glossary

Canonical language for Drover SQLForge. Shared actors (**customer**, **member**, **agent**) are defined in [`../CONTEXT-MAP.md`](../CONTEXT-MAP.md). Platform provisioning and hosted jobs: [`../drover-cloud/CONTEXT.md`](../drover-cloud/CONTEXT.md). Agent engine and MCP callers: [`../drover-code/CONTEXT.md`](../drover-code/CONTEXT.md). SQL safety policies for agent tool I/O: [`../drover-warden/CONTEXT.md`](../drover-warden/CONTEXT.md). Implementation details belong in `docs/` and ADRs, not here.

**Architecture decisions:** [`docs/adr/0001-no-jinja-policy.md`](docs/adr/0001-no-jinja-policy.md), [`docs/adr/0002-cli-invocation-drover-code-integration.md`](docs/adr/0002-cli-invocation-drover-code-integration.md), [`docs/adr/0003-preview-environment-ci.md`](docs/adr/0003-preview-environment-ci.md), [`docs/adr/0004-historized-snapshot.md`](docs/adr/0004-historized-snapshot.md).

## Product role

**Drover SQLForge**  
The **data automation engine** for a **data project**: pure-SQL transformations, structural dependency graphs, plan/apply execution, semantic metrics, and an MCP surface for **agents**. Distinct from **Drover Brain** (knowledge about code) and **Drover Code** (general agent tool loop). Not customer provisioning or multi-tenant SaaS in v1.

## Bounded context

**Data project automation**  
Everything SQLForge owns: parse models, fingerprint ASTs, plan changes, apply to **environments** on a configured warehouse, expose metrics and MCP tools. Scoped to one repo with `sqlforge.yml` + `models/`. Does not own **customer** lifecycle, Zitadel orgs, or Brain indexing.

## Actors

**Data engineer**  
Primary human actor for a **data project**. Owns the Git repo, runs the CLI/TUI, reviews **plans**, and approves **apply**. May also be a platform **member**, but SQLForge v1 does not require platform authentication.

**Agent (platform)**  
Automated actor that invokes SQLForge via MCP (`list_models`, `plan_change`, `apply_change`, etc.) on behalf of a **customer**. Uses the same plan/apply semantics as a **data engineer**; additional constraints come from **Drover Warden** (`scope: sqlforge`) and future **Drover Guard** bindings—not from a separate SQLForge tenancy model in v1.

## Out of scope (v1)

**Hosted SQLForge tenancy**  
Mapping a Drover **customer** or **tenant** to SQLForge **environments**, credentials, or warehouses. Future **Drover Cloud** integration; not part of the core glossary until that product path is specified.

## Data project

**Data project**  
One SQLForge workspace: `sqlforge.yml`, a `models/` tree, optional `models/semantic/*.yml`, and local state under `.sqlforge/`. The unit a **data engineer** or **agent** operates on—not a Drover **customer** (though a customer may own many data projects in Git).

**Warehouse connection**  
Project-level target warehouse from `sqlforge.yml` (`virtual:` dialect and connection). Shared by all **environments** in that project. Not an **environment** and not a Drover **tenant**.

## Models

**Model**  
One pure-SQL transformation defined by a `.sql` file under `models/`. Canonical user-facing term (CLI, MCP, docs). The Go type `Asset` is an implementation name only—never use *asset* in glossary or customer-facing docs.

**Model dependency**  
A directed edge in the **model DAG**: model B depends on model A when B’s SQL structurally references A’s output (WASM `ExtractRefs`, not Jinja `{{ ref() }}`).

## Environments

**Environment**  
A named plan/apply target within a **data project** (e.g. `prod`, `peter_dev`). What the CLI passes to `plan`, `apply`, and `query`. Distinct from a Drover **customer**, **tenant**, and from **warehouse connection**.

**Warehouse schema**  
The physical namespace on the warehouse for one **environment** (e.g. `sqlforge__peter_dev`). SQLForge injects this at apply time; **data engineers** reason in **environment** names, not raw schema strings.

**Base environment**  
The parent **environment** recorded as `base_env` when creating a child (typically `prod`). Used for lineage and zero-copy isolation semantics—not the same as `default_environment` in `sqlforge.yml` (which names the default target when the CLI omits an argument).

**Zero-copy isolation**  
How non-production **environments** materialize on supported warehouses (e.g. ClickHouse `CLONE`, views) without full physical copies. A materialization strategy documented in `docs/`; not a synonym for **environment**. Do not overload *virtual environment* in the glossary—the `virtual:` YAML key names **warehouse connection**, not an env.

## Plan and apply

**Plan**  
Compute an **execution plan** for an **environment** and present the diff. No warehouse writes. CLI: `sqlforge plan [environment]`. MCP: `plan_change` (deferred).

**Execution plan**  
The computed diff for one **environment**: **changed models**, **impacted models**, and **unchanged models**. Ephemeral—regenerated on every **plan** or **apply**; not persisted as a first-class artifact in v1 (CI logs and MCP responses may capture a snapshot).

**Apply**  
Regenerate the **execution plan**, then materialize every **changed** and **impacted** **model** in dependency order and update local state. The only warehouse mutation path in v1. CLI: `sqlforge apply [environment]`. MCP: `apply_change` (deferred). Non-TTY **apply** runs without confirmation; TTY may show progress UI.

**Changed model**  
A **model** whose **fingerprint** differs from the last successful **apply** recorded for that **model** in this **environment**.

**Impacted model**  
A downstream **model** that must rerun because an upstream **model** is **changed**, even when its own **fingerprint** is unchanged.

**Unchanged model**  
A **model** in the **execution plan** that is neither **changed** nor **impacted** for this **environment**.

**Fingerprint**  
Stable logical identity of a **model**: normalized AST plus model config (e.g. `-- @materialized:`). Whitespace and formatting-only SQL edits must not change the fingerprint. Stored per **model** per **environment** after **apply**.

**Run (deprecated)**  
Not a glossary term. The `sqlforge run` command is a stub; CI and **agents** use **plan** then **apply** instead.

## Semantic layer

**Semantic layer**  
The **metric** catalog for a **data project** (`models/semantic/*.yml`). Sits above **models**: business measures and dimensions without exposing raw warehouse table names to **agents**.

**Metric**  
One semantic definition: name, expression, allowed dimensions, and a backing **model**. Primary **agent** query primitive (`sqlforge query`, MCP `query_metric`). Distinct from a **model** file—metrics do not replace models; they compile against them.

**Metric query**  
Compile a **metric** (and optional dimension filters) to dialect-agnostic ANSI SQL scoped to an **environment**’s **warehouse schema**. Does not execute on the warehouse in v1—returns SQL only.

**Derived model**  
Synthetic **model** injected into the **model DAG** when a **metric** has `materialize: true` (naming pattern `semantic__{metric_name}`). A build artifact produced at plan/apply time, not a separate business noun in the glossary.

**Agent table blindness**  
**Agents** should satisfy analytics questions via **metric query** and semantic MCP tools, not by reading arbitrary **model** SQL or warehouse tables. **Data engineers** own **models** and **metrics** in Git. Enforced in v1 by tool surface design and **Drover Warden** (`scope: sqlforge`), not by SQLForge tenancy or row-level auth.

## Materialization and assertions

**Model config**  
Key/value pairs declared in SQL comment lines (`-- @key: value`) on a **model** file. Drives **materialization strategy**, incremental options, **data quality assertions**, and grain. Included in the **fingerprint**.

**Materialization strategy**  
The `materialized` entry in **model config**: how **apply** builds or updates the relation in the **warehouse schema** (`view`, `table`, `incremental`, `materialized_view`, or `streaming` via `kafka` / `nats` / `streaming`). Default when omitted: `view`.

**Materialized as**  
The **materialization strategy** last recorded in state for a **model** in an **environment** after a successful **apply**. Usually matches the configured strategy; **incremental** may differ on first run (create table vs merge).

**Data quality assertion**  
A warehouse check declared in **model config** (`test_not_null`, `test_unique`, `test_accepted_values_{column}`, `test_relationship`) or defined as custom SQL assertions in the `tests/` directory (Singular Tests) and executed during **apply** immediately after DDL (for model assertions) or at the end of DAG execution (for singular tests). A failed assertion aborts **apply**. CLI command: `sqlforge test [environment]`. Distinct from Go unit **tests** in the SQLForge repository.

## Incremental models

**Incremental model**  
A **model** whose **materialization strategy** is `incremental`.

**Initial build**  
The first **apply** for an **incremental model** in an **environment** when the physical table does not yet exist. Behaves like a **table** strategy (`CREATE TABLE`).

**Incremental run**  
A subsequent **apply** that runs **incremental merge** against the existing table.

**Incremental merge**  
The dialect-specific statement that loads the **model**’s SELECT into an existing table—append (`INSERT`), upsert (`ON CONFLICT`), or `MERGE`, depending on the active **warehouse connection** and **model config**.

**Unique key**  
**Model config** column(s) (`-- @unique_key:`) that identify a row for upsert/merge on warehouses that support it. When absent, runners default to append-only **incremental merge** where supported. On ClickHouse, **incremental merge** is append-only; deduplication is via table engine choice at **initial build**, not row-level `MERGE`.

**Incremental strategy**  
`-- @incremental_strategy:` — explicit control: `auto` (default: **merge** if **unique key** set, else **append**), `append`, `merge`/`upsert`, `delete+insert`, `replacing_merge_tree` (ClickHouse **initial build** only).

**Grain**  
`-- @grain:` — column(s) defining one logical output row of the **model**. Declared for semantics and included in the **fingerprint**; not enforced by **apply** in v1. May coincide with **unique key** but is not synonymous—used by future **historized snapshot** and incremental filtering work.

## Streaming models

**Streaming model**  
A **model** whose **materialization strategy** is `kafka`, `nats`, or `streaming`. **Apply** creates a broker-backed ingestion table in the **warehouse schema** (ClickHouse `ENGINE = Kafka` / `NATS`), not a SELECT pipeline rebuild.

**Batch model**  
A **model** materialized via SELECT logic (`view`, `table`, `incremental`, `materialized_view`). Default case; contrasts with **streaming model**. **Metric query** and most **agent** analytics assume batch **models** upstream.

**Stream connector config**  
**Model config** keys that configure the external broker (`kafka_broker_list`, `kafka_topic_list`, `nats_url`, `nats_subjects`, etc.) on a **streaming model**. Distinct from **warehouse connection** in `sqlforge.yml`.

**Streaming (deferred on dialect)**  
Only the ClickHouse runner generates real streaming DDL in v1. Other dialect runners return non-executable stubs; cross-dialect **streaming model** support is a **deferred capability**.

## Project layout and state

**Project root**  
The directory containing `sqlforge.yml`. The CLI and **local state store** resolve paths from the current working directory (expected to be the **project root**). Distinct from a Drover Code **workspace** (the broader Git tree an **agent** checks out).

**Project manifest**  
The `sqlforge.yml` file at the **project root**: project name/version, **default environment**, **warehouse connection**, and AI settings. A **data project** is the **project root** tree (manifest + `models/` + `.sqlforge/`).

**Project state**  
The gitignored `.sqlforge/` directory at the **project root**: **local state store**, MCP audit logs, and future runner plugin binaries. Machine-local cache of **apply** outcomes—not committed to Git and not the warehouse data plane.

**Local state store**  
Embedded SQLite (`state.db`) under **project state**. Records **environments**, per-**model** **fingerprints**, **materialized as**, and last **apply** timestamps. **Plan** compares live **models** against this store.

**Monorepo (deferred)**  
Multiple independent **data projects** inside one Git repository (separate manifests per subfolder). Not supported in v1—one **project root** per `sqlforge` invocation.

## Rebuild and schema change

**Full refresh (deferred)**  
An explicit rebuild of one or more **models** in an **environment**—drop/recreate or equivalent—bypassing **incremental merge** even when the table exists. Planned CLI surface (e.g. `apply --full-refresh`); not available in v1. Requires deliberate **data engineer** or CI action; **agents** must not trigger implicitly on **fingerprint** change alone.

**Schema drift**  
Mismatch between a **model**’s current SELECT output schema and the physical table already present in an **environment**. Resolution policy (fail **apply**, auto `ALTER`, or require **full refresh**) is not fixed in v1—document as an open operational concern.

**Initial build vs full refresh**  
**Initial build** creates the table the first time it is missing. **Full refresh** rebuilds when the table already exists. Both are distinct from a routine **incremental run**.

## CI and preview environments

**Preview environment**  
A short-lived **environment** for a pull request (e.g. `pr_123`), created from a **base environment** and materialized with **zero-copy isolation** when the **warehouse connection** supports it. Lets reviewers validate **plan** output against isolated **warehouse schema** data without touching `prod`.

**Base environment (CI)**  
The parent **environment** configured for a **preview environment** (typically `prod`). Same term as §Environments; in CI manifests as `base_environment`.

**CI apply gate (deferred)**  
Workflow policy where CI posts a **plan** on the PR but waits for human approval before **apply**. The v1 composite GitHub Action runs **plan** then **apply** automatically; an explicit gate is a future action input or organizational process. See [ADR 0003](docs/adr/0003-preview-environment-ci.md).

## Agent integration (MCP)

**SQLForge MCP server**  
SQLForge-owned HTTP JSON-RPC server started via `sqlforge mcp`. Exposes **data project** and warehouse tools to **agents**. Distinct from **Drover Brain** MCP (knowledge) and from **Drover Gateway** (which routes to registered MCP backends).

**MCP session environment**  
The **environment** bound when the **SQLForge MCP server** starts (e.g. `sqlforge mcp peter_dev`). All MCP tool calls operate in that context until the server restarts—not per-request env selection in v1.

**Agent-safe MCP tools (v1)**  
Default **agent** surface: `list_metrics`, `query_metric`, and optionally `list_models` (names/metadata only). Supports **agent table blindness** and **metric query** without warehouse mutation.

**plan_change / apply_change**  
MCP tools for proposed model edits: `plan_change` returns a `plan_id` and diff summary; `apply_change` executes that plan in the **MCP session environment**. Plans expire from the server store after two hours. Prefer human review before **apply_change** in production.

**get_model (restricted)**  
MCP tool that returns full **model** SQL and AST summary. Intended for **data engineer** debugging and lineage review, not the default **agent** path. **Drover Warden** may block or redact for **agents**.

## Principles

**Pure SQL model**  
A **model** authored as runnable warehouse SQL only—no templating preprocessor.

**Structural reference**  
A **model dependency** inferred by compile-time AST / table analysis (Polyglot WASM), not by macro expansion or `{{ ref() }}`.

**No-Jinja policy**  
SQLForge will not add Jinja, macro packages, or runtime string templating. Reusable logic belongs in warehouse UDFs, shared SQL in **SQL package import** (when shipped), or upstream **models**. Permanent product constraint—not a missing feature. See [ADR 0001](docs/adr/0001-no-jinja-policy.md).

**Model DAG**  
The directed acyclic graph of **models** (and **derived models**) in a **data project**. Drives topological **apply**, **impacted model** detection, and future export to external orchestrators. Prefer *model DAG* over *pipeline* or *dbt DAG* in SQLForge docs.

## Deferred capabilities and non-goals

**Deferred capability**  
A planned domain feature not yet shipped, documented so gaps are intentional. Current priority: **SQL package import**.

**Historized snapshot**  
SCD Type 2 change tracking for a source relation (dbt *snapshot* analogue). Defined in `snapshots/*.sql`; applied via `sqlforge snapshot [environment]`. Injects `sqlforge_valid_from` / `sqlforge_valid_to`. Distinct from an **execution plan** snapshot captured in CI logs or MCP responses. See [ADR 0004](docs/adr/0004-historized-snapshot.md).

**Snapshot definition**  
A pure SQL file under `snapshots/` with `-- @strategy:`, `-- @unique_key:` (or **grain**), and `-- @updated_at:` for timestamp strategy. Not a **model** in the **model DAG**.

**Snapshot run**  
One execution of `sqlforge snapshot` for a **snapshot definition** in an **environment** (**initial build** or timestamp **incremental run**).

**Column lineage**  
Column-level provenance from model SQL (`sqlforge lineage [model]`; `column_lineage` on **get_model** MCP). v1 uses structural SELECT/FROM parsing; WASM AST enrichment is future work. Strict superset of table-level **model dependencies**.

**SQL package import**  
Import of remote Git repositories containing pure SQL UDFs and/or `metrics.yml` into a **data project**—not macro/Jinja packages.

**Non-goal**  
Out of scope by design for SQLForge: Jinja/macros, Python/PySpark **models**, built-in job scheduler (orchestration stays external—GitHub Actions, Airflow, etc.).

## Ecosystem integration

**Workspace-bound data project**  
SQLForge runs against the Git workspace the **agent** or **data engineer** already has checked out. Requires `sqlforge.yml` in that tree. No platform-provisioned SQLForge service or per-**customer** warehouse binding in v1.

**CLI invocation**  
**Agent** or CI executes `sqlforge` as a subprocess (`plan`, `apply`, `query`, etc.) in the workspace—the primary v1 integration path with **Drover Code**. Drover Code detects `sqlforge.yml` and injects CLI guidance (`drover-code/internal/integrations/sqlforge`; how-to: [`../drover-code/docs/how-to/sqlforge-from-drover-code.md`](../drover-code/docs/how-to/sqlforge-from-drover-code.md)). Same semantics as a local **data engineer**; **Drover Warden** `scope: sqlforge` policies apply to tool I/O. See [ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md).

**Sidecar MCP**  
A **SQLForge MCP server** process colocated with the **agent** (dev machine or **worker instance**), bound to one **MCP session environment**. The **agent** reaches it via **Drover Gateway** MCP routing when configured. Complements **CLI invocation**; does not replace Git-backed **data project** state.

**Deferred platform integration**  
Future: **Drover Muster** discovers SQLForge MCP tools, **Drover Guard** gates destructive **apply**, **hosted SQLForge tenancy** maps **customers** to warehouses. v1: **CLI invocation** or **sidecar MCP** plus **Warden** only.

**Brain vs SQLForge split**  
**Drover Brain** MCP answers repository knowledge (design, code graph). **SQLForge MCP server** answers **metrics**, **model DAG**, and warehouse **plan**/**apply**. An **agent** may use both when the workspace includes a **data project**; the servers stay separate—never conflate Brain search with **metric query**.
