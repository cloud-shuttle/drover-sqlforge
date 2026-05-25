# Drover SQLForge 🚀 (v0.1.0-alpha)

> Part of the [Drover Ecosystem](../DROVER_ECOSYSTEM.md) — Orchestrating Autonomous Agentic Engineering

**SQLForge** is a modern, pure Go-native alternative to dbt. It replaces Jinja templating with **compile-time AST analysis** via an embedded Polyglot WASM engine, giving you repeatable, agent-friendly data transformations across every major SQL warehouse.

---

## Why SQLForge?

| | dbt | SQLMesh | **SQLForge** |
|---|---|---|---|
| Templating | Jinja (runtime) | Jinja (runtime) | **Pure SQL — AST at compile time** |
| Environments | Profiles | Environments | **Named envs + zero-copy isolation** |
| Agentic integration | dbt Cloud API | OSS MCP (experimental) | **First-class MCP server + CLI** |
| Binary | Python + venv | Python + venv | **Single Go binary** |
| Plugin model | Adapters in Python | Adapters in Python | **gRPC plugin binaries** |
| Data quality | YAML tests | YAML tests | **YAML + singular SQL + relationship tests** |

---

## Core Philosophy

`sqlforge` diverges from legacy data modeling tools by prioritizing **compile-time AST analysis** over runtime string templating (like Jinja). See [ADR 0001](docs/adr/0001-no-jinja-policy.md).

1. **Jinja-free**: **Models** are pure SQL. No `{{ ref() }}` blocks. **Structural references** resolve dependencies at parse time via the embedded WASM parser.
2. **Environments**: Named plan/apply targets (`prod`, `peter_dev`) with isolated **warehouse schemas**. Non-prod targets use **zero-copy isolation** (e.g. ClickHouse `CLONE`) where supported.
3. **Plan & apply**: Terraform-style workflow. **Fingerprints** (AST + model config) detect logical changes, not formatting noise.
4. **Agent-ready**: Semantic **metrics**, MCP read tools, and CLI **plan**/**apply** for **Drover Code** agents ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).
5. **Lightweight core**: The `sqlforge` binary has no CGO or warehouse drivers. Warehouse connectivity is handled by standalone **gRPC plugin binaries** (`sqlforge-plugin-<dialect>`).

---

## Supported Warehouses

| Warehouse | Plugin | Status |
|-----------|--------|--------|
| ClickHouse | built-in | ✅ Production |
| DuckDB | `sqlforge-plugin-duckdb` | ✅ Production |
| Snowflake | `sqlforge-plugin-snowflake` | ✅ Production |
| Databricks | `sqlforge-plugin-databricks` | ✅ Production |
| PostgreSQL | `sqlforge-plugin-postgres` | ✅ Production |
| Apache Doris | `sqlforge-plugin-doris` | ✅ Alpha |
| VeloDB (SelectDB) | `sqlforge-plugin-velodb` | ✅ Alpha |

---

## Installation

```bash
git clone https://github.com/drover-org/drover-sqlforge.git
cd drover-sqlforge

# Build the core CLI
go build -o sqlforge ./cmd/sqlforge

# Or build everything (CLI + all warehouse plugins)
make build
make plugins
```

> **Note:** `make build` also compiles the optional React Web GUI (`ui/`), which requires Node 20+. If you only want the CLI, `go build -o sqlforge ./cmd/sqlforge` is sufficient.

---

## Quick Start

Run the bundled `agentic_retail_2026` example project against DuckDB (no external services needed):

```bash
cd examples/agentic_retail_2026

# Pull external SQL package dependencies
../../sqlforge deps

# Parse models and visualise the DAG
../../sqlforge parse

# Create an isolated dev environment
../../sqlforge env create my_dev

# Preview what will change
../../sqlforge plan my_dev

# Apply the changes
../../sqlforge apply my_dev

# Run all data quality assertions
../../sqlforge test my_dev

# Column-level lineage for a model
../../sqlforge lineage customer_360

# Generate the offline data catalog
../../sqlforge docs generate my_dev
open target/index.html

# Start the local Web GUI
../../sqlforge ui my_dev
```

---

## Key Commands

| Command | Description |
|---------|-------------|
| `sqlforge parse` | Parse models, show DAG and fingerprints |
| `sqlforge plan [env]` | Show what will change (no DB writes) |
| `sqlforge apply [env]` | Materialise models in dependency order |
| `sqlforge test [env]` | Run all data quality assertions |
| `sqlforge lineage [model]` | Column-level upstream/downstream lineage |
| `sqlforge snapshot [env]` | Apply SCD Type 2 historized snapshots |
| `sqlforge docs generate [env]` | Compile offline HTML data catalog |
| `sqlforge ui [env]` | Start local Web GUI with live DAG viewer |
| `sqlforge mcp [env]` | Start MCP server for agentic integrations |
| `sqlforge env create [name]` | Create an isolated warehouse environment |
| `sqlforge ai explain [model]` | LLM-powered model explanation (Ollama) |

---

## Data Quality Testing

SQLForge supports three tiers of data quality assertions, all evaluated at `apply` time with zero external tooling:

```sql
-- models/stg_orders.sql

-- @test_not_null: order_id
-- @test_unique: order_id
-- @test_accepted_values: status | pending,shipped,delivered,cancelled
-- @test_relationship: customer_id to stg_customers.customer_id
SELECT ...
```

**Singular tests** — custom SQL in `tests/` that fail if they return any rows:
```sql
-- tests/no_negative_revenue.sql
SELECT order_id FROM stg_orders WHERE revenue < 0
```

---

## Static Data Catalog

`sqlforge docs generate [env]` compiles the entire model DAG, column-level lineage, semantic metrics, data quality assertions, and model configs into a **single self-contained HTML file** — no server required.

```bash
./sqlforge docs generate prod
open target/index.html
```

The catalog supports offline browsing via the `file://` protocol (zero CORS), making it trivially shareable via S3, GitHub Pages, or email attachment.

---

## Semantic Metrics Layer

Define warehouse-agnostic metrics in `metrics.yml`:

```yaml
metrics:
  - name: daily_active_users
    label: Daily Active Users
    model: daily_metrics
    expression: COUNT(DISTINCT user_id)
    dimensions: [metric_date, country]
```

Query via CLI or MCP:
```bash
./sqlforge query daily_active_users --filter "country='AU'"
```

---

## Agentic Integration (MCP)

Start the MCP server for any **Drover Code** agent or compatible IDE:

```bash
./sqlforge mcp prod
```

| Tool | Description |
|------|-------------|
| `list_models` | Model names, config, fingerprints |
| `get_model` | Full model SQL + column lineage |
| `list_metrics` | Discover semantic metrics |
| `query_metric` | Execute a metric query (compiled SQL only) |
| `plan_change` | Propose SQL for a model → `plan_id` |
| `apply_change` | Execute a `plan_id` (2h TTL) |

---

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full breakdown. In brief:

- **WASM parser** — `internal/parser/polyglot.wasm` via `wazero` for zero-CGO AST extraction
- **gRPC plugins** — `cmd/plugins/sqlforge-plugin-*` handle all warehouse connections; the core binary stays CGO-free
- **State** — `.sqlforge/state.db` (SQLite) tracks environments, fingerprints, and apply history
- **Concurrent DAG engine** — models execute in parallel with sequential schema pre-creation to avoid write-write conflicts

---

## Documentation

Docs under `docs/` follow [Diátaxis](https://diataxis.fr/):

- [`docs/adr/`](docs/adr/) — Architecture Decision Records
- [`docs/explanation/`](docs/explanation/) — Concept deep-dives
- [`docs/how-to/`](docs/how-to/) — Step-by-step task guides
- [`docs/reference/`](docs/reference/) — CLI and config reference
- [`docs/tutorials/`](docs/tutorials/) — End-to-end walkthroughs

New pages require YAML frontmatter per the org [content taxonomy](../docs/taxonomy.yaml); validate with [`scripts/validate-content-frontmatter.sh`](scripts/validate-content-frontmatter.sh).

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, testing strategy, and PR guidelines.

Key commands:
```bash
go test ./...          # Unit tests
make e2e               # Fast end-to-end CLI tests (DuckDB, milliseconds)
make integration       # Live ClickHouse integration tests (Docker required)
make plugins           # Compile all warehouse plugin binaries
```

See [ARCHITECTURE.md](ARCHITECTURE.md), [ROADMAP.md](ROADMAP.md), [CONTEXT.md](CONTEXT.md) (domain glossary), and [docs/adr/](docs/adr/) for architecture decisions.

---

## License

Apache 2.0 — see [LICENSE](LICENSE).