# Tutorial: Your First SQLForge Project

**Time:** 20–30 minutes  
**What you'll learn:** How to install SQLForge, understand the project structure, and run your first `plan` and `apply` against the bundled example project.  
**What you'll have:** A working SQLForge installation and a mental model of the core plan/apply workflow.

---

## Before you begin

You'll need:

- **Go 1.22+** — check with `go version`
- **Git**
- No database server required — this tutorial uses DuckDB, which runs entirely in-memory.

> **New to data transformation tools?** SQLForge works like Terraform, but for SQL. You write models (`.sql` files), SQLForge figures out the right order to build them, and `apply` executes them safely in your warehouse. There is no Python, no `{{ ref() }}`, and no virtual environments to manage.

---

## Step 1: Install SQLForge

Clone the repository and build the CLI:

```bash
git clone https://github.com/drover-org/drover-sqlforge.git
cd drover-sqlforge
make build
make plugins
```

`make build` compiles the core `sqlforge` binary. `make plugins` compiles the warehouse drivers — standalone binaries that keep the core CLI lightweight.

Verify the installation:

```bash
./sqlforge --version
# sqlforge version v0.1.0-alpha
```

> **Tip:** Add `sqlforge` to your `$PATH` with `sudo cp sqlforge /usr/local/bin/` so you can call it from any project directory. For now, we'll use `./sqlforge` from the repo root.

---

## Step 2: Explore the example project

SQLForge ships with a complete example project. Let's look at it:

```bash
cd examples/agentic_retail_2026
ls models/
# intermediate/  marts/  semantic/  staging/
```

This is a typical four-layer project structure. Open `sqlforge.yml`:

```bash
cat sqlforge.yml
```

```yaml
name: agentic_retail_2026
version: 1.0.0
default_environment: prod

virtual:
  dialect: duckdb
  connection: memory    # in-memory DuckDB — no server needed
  default_type: virtual
```

This file tells SQLForge how to connect to your warehouse. `connection: memory` means everything runs in-memory — perfect for learning.

Now open one of the models:

```bash
cat models/staging/stg_users.sql
```

Notice the config comments at the top:

```sql
-- @materialized: table
-- @test_not_null: user_id
```

This is how SQLForge models work: **pure SQL, zero Jinja**. The `-- @` comments are model configuration. SQLForge reads your SQL at parse time to understand dependencies — no `{{ ref("stg_users") }}` needed.

---

## Step 3: Parse the project

Before running anything, ask SQLForge to parse and validate the project:

```bash
../../sqlforge parse
```

```
Parsing models...
Found 8 models across 4 layers
DAG validated — no cycles detected
```

SQLForge has read all your `.sql` files, built a dependency graph (DAG), and confirmed there are no circular references. Nothing has been written to your warehouse yet.

---

## Step 4: Preview the plan

Ask SQLForge what it _would_ do, without doing anything:

```bash
../../sqlforge plan prod
```

```
Execution Plan:
  Changed Models: 8
    - stg_events
    - stg_experiments
    - stg_users
    - int_customer_sessions
    - ab_test_results
    - customer_360
    - daily_metrics
    - semantic__daily_active_users
  Impacted Models: 0
  Unchanged Models: 0
```

This is the Terraform-style plan. Since this is a fresh project, all 8 models are "changed" (they haven't been applied yet). The order shown is the topological execution order — SQLForge always builds dependencies before the models that depend on them.

Nothing has been written to your warehouse yet. `plan` is always read-only.

---

## Step 5: Apply the plan

Now execute it:

```bash
../../sqlforge apply prod
```

You'll see each model applied in order:

```
[1/8] Applying stg_events...        ✓ 42ms
[2/8] Applying stg_experiments...   ✓ 18ms
[3/8] Applying stg_users...         ✓ 12ms
...
[8/8] Applying semantic__daily_active_users... ✓ 31ms

Apply completed successfully. (8 models in 0.3s)
```

SQLForge has:
1. Created a warehouse schema called `sqlforge__prod`
2. Built each model in dependency order
3. Run any data quality tests declared in the model config
4. Stored the AST fingerprint of each model in `.sqlforge/state.db`

---

## Step 6: Apply again and watch SQLForge skip unchanged models

Run apply a second time without changing anything:

```bash
../../sqlforge apply prod
```

```
Execution Plan:
  Changed Models: 0
  Impacted Models: 0
  Unchanged Models: 8

Nothing to apply.
```

SQLForge compares the AST fingerprint of each model against what was last applied. Since nothing changed, it skips everything. This is **fingerprint-based incrementality** — formatting changes, added comments, and whitespace are all ignored. Only real logical changes trigger a rebuild.

---

## What you've learned

You've seen the core SQLForge workflow end-to-end:

| Command | What it does |
|---------|-------------|
| `sqlforge parse` | Validates models and builds the DAG |
| `sqlforge plan <env>` | Shows what would change, without touching the warehouse |
| `sqlforge apply <env>` | Executes the plan safely, in dependency order |

You've also seen that:
- Models are **pure SQL** — no templating language
- Dependencies are **structural** — SQLForge detects them from your SQL at parse time
- Apply is **fingerprint-driven** — unchanged models are never rebuilt

---

## Next steps

- **[Tutorial: Write your first model](02-your-first-model.md)** — create a new model from scratch and see SQLForge detect the dependency automatically.
- **[How-to: Create and use environments](../how-to/02-environments.md)** — create a personal dev environment isolated from production.
- **[Explanation: Why no Jinja?](../../docs/adr/0001-no-jinja-policy.md)** — the thinking behind pure-SQL models and compile-time AST analysis.
