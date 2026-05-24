# Drover SQLForge 🚀 (v0.1.0-alpha)

> Part of the [Drover Ecosystem](../DROVER_ECOSYSTEM.md) — Orchestrating Autonomous Agentic Engineering

A modern, fast, and pure Go-native alternative to dbt, powered by **Polyglot WASM** for deep SQL intelligence, isolated **environments**, and **zero-copy isolation** on supported warehouses.

## Core Philosophy

`sqlforge` diverges from legacy data modeling tools by prioritizing **compile-time AST analysis** over runtime string templating (like Jinja). See [ADR 0001](docs/adr/0001-no-jinja-policy.md).

1. **Jinja-free**: **Models** are pure SQL. No `{{ ref() }}` blocks. **Structural references** resolve dependencies at parse time.
2. **Environments**: Named plan/apply targets (`prod`, `peter_dev`) with isolated **warehouse schemas**. Non-prod targets use **zero-copy isolation** (e.g. ClickHouse `CLONE`) where supported.
3. **Plan & apply**: Terraform-style workflow. **Fingerprints** (AST + model config) detect logical changes, not formatting noise.
4. **Agent-ready**: Semantic **metrics**, MCP read tools, and CLI **plan**/**apply** for **Drover Code** agents ([ADR 0002](docs/adr/0002-cli-invocation-drover-code-integration.md)).

## ✨ Recent Updates (v0.1.0-alpha)
- **WASM AST Transpilation**: Automatically transpile SQL across dialects (e.g., Snowflake to DuckDB) during plan execution using an embedded WASM engine.
- **Orchestrator Export Hooks**: Export your full DAG and execution commands to standard JSON (`sqlforge export dag`) for Airflow/Dagster integration.
- **Pure-SQL Package Manager**: Import remote Git repositories natively in `packages.yml` and resolve dependencies automatically.
- **Custom Targets & Folder Capture**: Route models dynamically to specific schemas and databases based on folder structures (e.g., `models/marts/marketing`).

## Installation

```bash
git clone https://github.com/drover-org/drover-sqlforge.git
cd drover-sqlforge
make cli
```

## Quick Start

Check out the bundled example project:
```bash
cd examples/agentic_retail_2026

# Pull external dependencies from packages.yml
../../sqlforge deps

# Parse the models and see the DAG
../../sqlforge parse

# Create an environment (isolated warehouse schema)
../../sqlforge env create peter_dev

# See what will change (execution plan)
../../sqlforge plan peter_dev

# Apply changes to that environment
../../sqlforge apply peter_dev

# Historized snapshot (SCD Type 2) from snapshots/
../../sqlforge snapshot prod

# Column-level lineage for a model
../../sqlforge lineage customer_360

# Use local AI to explain a model
../../sqlforge ai explain customer_360
```

## Architecture

- **wazero**: Embeds a Rust-based Polyglot WASM module for structural SQL analysis and reference extraction.
- **Project state**: Gitignored `.sqlforge/state.db` tracks **environments**, **fingerprints**, and last **apply** per **model**.
- **Warehouse connection**: `sqlforge.yml` `virtual:` block sets dialect and connection (not an environment name).
- **Cobra CLI**: Primary interface; **SQLForge MCP server** for read-heavy agent flows (`sqlforge mcp`).
- **Web GUI** (`ui/`): Optional; npm is **build-time only**—see [`ui/SECURITY.md`](ui/SECURITY.md).

## Documentation

Docs under `docs/` follow [Diátaxis](https://diataxis.fr/). New pages require YAML frontmatter per the org [content taxonomy](../docs/taxonomy.yaml); validate with [`scripts/validate-content-frontmatter.sh`](scripts/validate-content-frontmatter.sh).

## Contributing

We welcome contributions! If you want to run the tests, fuzzers, or understand how the WASM polyglot is integrated, please read our [Contributing Guide](CONTRIBUTING.md).

See [ARCHITECTURE.md](ARCHITECTURE.md), [ROADMAP.md](ROADMAP.md), [CONTEXT.md](CONTEXT.md) (domain glossary), and [docs/adr/](docs/adr/) for architecture decisions.