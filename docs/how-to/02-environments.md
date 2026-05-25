# How-to: Create and Use Environments

**When to use this:** You want to work on model changes without affecting production data, or you want each CI pull request to get its own isolated schema.

---

## What environments are

An **environment** is a named deployment target. It maps to an isolated warehouse schema: `sqlforge__<name>`.

When you run `sqlforge apply prod`, SQLForge writes to `sqlforge__prod`. When you run `sqlforge apply alice_dev`, it writes to `sqlforge__alice_dev`. The two schemas never interfere with each other.

On supported warehouses (ClickHouse), non-prod environments are created as **zero-copy clones** of the base environment — meaning you get a fully isolated copy of production data instantly, without duplicating storage.

---

## Create a personal dev environment

```bash
# Inside your project directory
sqlforge env create alice_dev
```

SQLForge creates `sqlforge__alice_dev` in your warehouse. If your warehouse supports zero-copy cloning (ClickHouse), all tables from `prod` are cloned instantly. For other warehouses, the schema is created empty and populated on first `apply`.

### Clone from a specific base environment

By default, new environments clone from the project's `default_environment` (usually `prod`). To clone from a different base:

```bash
sqlforge env create alice_dev --base-env staging
```

---

## Plan and apply against your environment

Once the environment exists, use it exactly like `prod`:

```bash
sqlforge plan alice_dev      # see what has changed since last apply
sqlforge apply alice_dev     # execute against sqlforge__alice_dev
```

Your changes go to `sqlforge__alice_dev`. Production is untouched.

---

## Use environments in CI

The standard pattern is one ephemeral environment per pull request:

```yaml
# .github/workflows/ci.yml (extract)
- name: Create PR environment
  run: sqlforge env create pr_${{ github.event.pull_request.number }} --base-env prod

- name: Plan
  run: sqlforge plan pr_${{ github.event.pull_request.number }}

- name: Apply
  run: sqlforge apply pr_${{ github.event.pull_request.number }}
```

See the [ADR on preview environments in CI](../../docs/adr/0003-preview-environment-ci.md) for the full design rationale.

---

## How schema naming works

| Environment name | Warehouse schema |
|-----------------|-----------------|
| `prod` | `sqlforge__prod` |
| `staging` | `sqlforge__staging` |
| `alice_dev` | `sqlforge__alice_dev` |
| `pr_42` | `sqlforge__pr_42` |

The prefix `sqlforge__` is always prepended. You cannot configure this prefix — it ensures SQLForge-managed schemas are visually distinct from unmanaged tables in your warehouse.

---

## Cross-database models

If a model declares `-- @database: analytics`, SQLForge resolves its target as `analytics.sqlforge__<env>.<model>` rather than `sqlforge__<env>.<model>`. This lets you organise models across multiple warehouse databases while keeping environment isolation intact.

See [model config reference](../reference/model-config.md) for `@database` and `@schema`.

---

## Troubleshooting

**`env create` fails with "schema already exists"**
The environment was created before (possibly by a previous CI run). SQLForge treats this as a no-op for most warehouses — `CREATE SCHEMA IF NOT EXISTS` is idempotent.

**I see data from prod in my dev environment (ClickHouse)**
This is expected — zero-copy clones share the underlying data blocks with the source at clone time. If you apply changes, the clone diverges and stops sharing those blocks. To reset to prod, drop and recreate the environment.

**Changes in `alice_dev` are appearing in `plan prod`**
They won't. Plans are always scoped to a single environment. `plan prod` compares against the fingerprints stored for `prod`; `plan alice_dev` compares against `alice_dev` fingerprints. They are completely independent.

---

## Related

- [Reference: sqlforge.yml](../reference/sqlforge-yml.md) — `default_environment`, `virtual.connection`
- [ADR 0003: Preview environments in CI](../../docs/adr/0003-preview-environment-ci.md)
