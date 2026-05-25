# Tutorial: Write Your First Model

**Time:** 15–20 minutes  
**Prerequisites:** Complete [Tutorial 1: Your First SQLForge Project](01-getting-started.md) first.  
**What you'll learn:** How to create a new model, declare its materialization, let SQLForge detect its dependency automatically, add a data quality test, and see only your new model rebuild on apply.  
**What you'll have:** A new model in the example project, and confidence in the full author → plan → apply loop.

---

## The scenario

The example project has a `stg_users` staging model. You're going to add a new model — `active_users` — that filters `stg_users` down to users who signed up in the last 90 days. It's a simple mart model that demonstrates everything important about how SQLForge works.

---

## Step 1: Create the model file

Navigate to the example project:

```bash
cd examples/agentic_retail_2026
```

Create a new file in `models/marts/`:

```bash
cat > models/marts/active_users.sql << 'EOF'
-- @materialized: table
-- @test_not_null: user_id

SELECT
    user_id,
    email,
    signup_date,
    country
FROM stg_users
WHERE signup_date >= CURRENT_DATE - INTERVAL 90 DAY
EOF
```

That's the entire model. Let's understand what each part does:

| Line | What it does |
|------|-------------|
| `-- @materialized: table` | Tells SQLForge to build this as a physical table, not a view |
| `-- @test_not_null: user_id` | Declares a data quality assertion: `user_id` must never be null |
| `FROM stg_users` | SQLForge will detect this as a dependency automatically |

Notice there is no `{{ ref("stg_users") }}`. You write plain SQL. SQLForge's WASM parser reads your file at parse time and builds the dependency graph structurally.

---

## Step 2: Parse to verify SQLForge sees the dependency

```bash
../../sqlforge parse
```

```
Parsing models...
Found 9 models across 4 layers

Model: active_users
  Path:          models/marts/active_users.sql
  Materialized:  table
  Dependencies:  [stg_users]
  Tests:         [not_null(user_id)]
```

SQLForge found your model, detected its dependency on `stg_users`, and registered the `not_null` test — all from reading the SQL file. Nothing was written to the warehouse.

---

## Step 3: Plan — see only your model in the diff

```bash
../../sqlforge plan prod
```

```
Execution Plan:
  Changed Models: 1
    - active_users
  Impacted Models: 0
  Unchanged Models: 8
```

This is the key behaviour to understand: SQLForge compared the AST fingerprint of every model against the state stored in `.sqlforge/state.db`. The 8 existing models haven't changed, so they show as "Unchanged". Only your new model appears in "Changed".

`stg_users` appears as an Unchanged model, not a Changed one — SQLForge won't rebuild it. But `active_users` depends on it, so SQLForge knows `stg_users` must already exist before `active_users` can be built.

---

## Step 4: Apply

```bash
../../sqlforge apply prod
```

```
[1/1] Applying active_users...
  → CREATE OR REPLACE TABLE sqlforge__prod.active_users AS
    SELECT user_id, email, signup_date, country
    FROM sqlforge__prod.stg_users
    WHERE signup_date >= CURRENT_DATE - INTERVAL 90 DAY
  ✓ Table created
  ✓ not_null(user_id) — passed (0 violations)
  ✓ Fingerprint saved

Apply completed successfully. (1 model in 0.1s)
```

Three things happened in sequence:

1. **Transpilation** — SQLForge rewrote `stg_users` to `sqlforge__prod.stg_users`, safely injecting your environment schema using the token-aware SQL rewriter (not regex).
2. **Execution** — the DDL ran against DuckDB in-memory.
3. **Quality assertion** — the `not_null(user_id)` test ran as `SELECT COUNT(*) FROM sqlforge__prod.active_users WHERE user_id IS NULL`. It found 0 violations, so apply succeeded.

---

## Step 5: See what happens when a quality test fails

Let's deliberately break it. Edit the model to introduce a null:

```bash
cat > models/marts/active_users.sql << 'EOF'
-- @materialized: table
-- @test_not_null: user_id

SELECT
    NULL AS user_id,    -- deliberately broken
    email,
    signup_date,
    country
FROM stg_users
WHERE signup_date >= CURRENT_DATE - INTERVAL 90 DAY
EOF
```

Apply again:

```bash
../../sqlforge apply prod
```

```
[1/1] Applying active_users...
  ✗ data quality test failed: sqlforge__prod.active_users.user_id is not_null
    but found 150 null records

Apply failed. The model was created but the quality gate blocked completion.
```

SQLForge enforces quality gates as part of apply — not as a separate step. The model was built but the apply is marked as failed, and the fingerprint is _not_ saved to state. If you run `plan` again, `active_users` will still appear as a Changed model.

Restore the working version:

```bash
cat > models/marts/active_users.sql << 'EOF'
-- @materialized: table
-- @test_not_null: user_id

SELECT
    user_id,
    email,
    signup_date,
    country
FROM stg_users
WHERE signup_date >= CURRENT_DATE - INTERVAL 90 DAY
EOF
../../sqlforge apply prod
```

---

## Step 6: View the lineage

```bash
../../sqlforge lineage active_users
```

```
active_users
  ← stg_users (table)
       ← [source: system.numbers]
```

SQLForge traces the full upstream lineage using column-level AST analysis. You can also see this visually:

```bash
../../sqlforge ui prod
# Open http://localhost:8080 and click the active_users node
```

---

## What you've learned

You've seen the full model authoring loop:

1. Write a `.sql` file with `-- @` config comments
2. `parse` — SQLForge builds the dependency graph from your SQL
3. `plan` — only changed models appear in the diff; unchanged models are skipped
4. `apply` — builds in dependency order, runs quality tests, saves fingerprint
5. Failed quality tests block the fingerprint save — the model is re-applied next time

---

## Key things to remember

- **No `{{ ref() }}`** — write `FROM stg_users`, not `FROM {{ ref("stg_users") }}`. SQLForge detects it.
- **Config via comments** — `-- @materialized:`, `-- @test_not_null:`, `-- @unique_key:`, etc. See the [model config reference](../reference/model-config.md).
- **Fingerprint-driven** — only logical changes trigger a rebuild. Reformatting your SQL won't.
- **Quality gates are synchronous** — they run as part of `apply`, not separately.

---

## Next steps

- **[How-to: Create and use environments](../how-to/02-environments.md)** — create a `dev` environment isolated from `prod` so you can iterate safely.
- **[How-to: Write data quality tests](../how-to/data-quality-testing.md)** — `unique`, `accepted_values`, and cross-model `relationship` tests.
- **[Reference: Model config keys](../reference/model-config.md)** — every `-- @` directive and what it does.
