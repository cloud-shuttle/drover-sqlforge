# ADR 0005: Pure-SQL Package Management System

## Status
Accepted

## Context
SQLForge intentionally avoids Jinja templating in favor of pure SQL and AST parsing (see [ADR 0001](0001-no-jinja-policy.md)). As a result, SQLForge lacks "macros" and dynamic `{{ ref('package', 'model') }}` scoping. However, large enterprise teams still need a mechanism to share reusable models, staging logic, and semantic metrics (`metrics.yml`) across different data projects without duplicating files.

We need a lightweight dependency management system similar to `dbt deps` but adapted for pure SQL constraints.

## Decision
We will implement a simple package management system based on `packages.yml` and Git operations:

1. **`packages.yml`**: A configuration file at the root of a SQLForge project that specifies external Git repositories and specific revisions (branch, tag, or commit hash).
2. **`sqlforge deps`**: A CLI command that reads `packages.yml`, clones the specified repositories into a local `sqlforge_packages/` directory, and checks out the requested revision.
3. **Runtime Integration**: During execution (`sqlforge plan` / `sqlforge apply`), the engine automatically scans `sqlforge_packages/`. Any models (`models/*.sql`) and semantic metrics (`metrics.yml`) found inside packages are aggregated into the global runtime execution DAG.

### Constraint: Global Namespace
Because we do not have Jinja, models cannot be dynamically renamed or explicitly namespaces during compilation via macros. Therefore, we enforce **global uniqueness** for model names. A model named `stg_events` in an imported package will clash with a local model named `stg_events`. For v1, package authors are expected to prefix their models (e.g. `pkg_name_stg_events`) or users must ensure no naming collisions occur.

## Consequences

### Positive
* Teams can share best-practice models, utility tables, and core semantic metrics across the organization.
* The system remains simple, relying entirely on standard Git mechanics rather than an external registry.
* No templating engine is required, preserving SQLForge's compile-time AST verification strategy.

### Negative
* The lack of dynamic macros limits how configurable imported models can be.
* Users must manage global namespace collisions manually.
