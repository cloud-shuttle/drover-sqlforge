# Architecture

`sqlforge` is a Go-native SQL transformation engine focusing on:
1. Pure SQL Models
2. Zero-Copy Virtual Environments
3. AST-based fingerprinting using Polyglot WASM.

## Components
- **CLI**: Powered by Cobra.
- **Parser**: Uses `wazero` to load a Rust-compiled Polyglot WASM module.
- **Graph**: Builds a DAG of dependencies and calculates stable fingerprints.
- **State**: Embedded SQLite database tracking models and environments.
- **Plan & Apply Engine**: Computes minimum changes required based on state and runs execution tasks.
