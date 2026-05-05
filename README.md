# Drover SQLForge 🚀 (v0.1.0-alpha)

A modern, fast, and pure Go-native alternative to dbt, powered by **Polyglot WASM** for deep SQL intelligence and zero-copy virtual environments.

## Core Philosophy

`sqlforge` diverges from legacy data modeling tools by prioritizing **compile-time AST analysis** over runtime string templating (like Jinja). 

1. **Jinja-Free**: Models are written in pure SQL. No `{{ ref() }}` blocks. References are resolved structurally.
2. **Virtual Environments**: Uses ClickHouse `CLONE` (zero-copy) for instant, isolated staging environments.
3. **Plan & Apply Workflow**: Terraform-style planning. Uses AST structural hashing (fingerprints) to detect exactly what logical code changed, ignoring formatting.
4. **Agentic Ready**: Built-in AI hooks (`sqlforge ai explain`) to interpret models and connect to the semantic layer natively.

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

# Parse the models and see the DAG
../../sqlforge parse

# Create a virtual environment
../../sqlforge env create peter_dev

# See what will change
../../sqlforge plan peter_dev

# Apply changes to your environment
../../sqlforge apply peter_dev

# Use local AI to explain a model
../../sqlforge ai explain customer_360
```

## Architecture

- **wazero**: Embeds a Rust-based Polyglot WASM module for high-speed SQL transpilation and reference extraction.
- **SQLite State**: Locally tracks applied environments and model fingerprints (`.sqlforge/state.db`).
- **Cobra CLI**: The user-facing command structure.

## Contributing

We welcome contributions! If you want to run the tests, fuzzers, or understand how the WASM polyglot is integrated, please read our [Contributing Guide](CONTRIBUTING.md).

See [ARCHITECTURE.md](ARCHITECTURE.md) and [ROADMAP.md](ROADMAP.md) for more details.