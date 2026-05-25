# Advanced Data Quality Testing in SQLForge 🛡️

SQLForge provides a robust, native data quality engine to validate that your warehouse relations conform to structural rules and business assumptions. Tests are executed immediately after DDL materialization during `sqlforge apply` (aborting the pipeline on failure) or standalone via `sqlforge test`.

This guide walks through the three types of data quality assertions supported by SQLForge and how to execute them.

---

## 1. Column Assertions (Schema Tests)

Column assertions are declared directly inside your model's SQL file header as configuration comments.

### 🚫 Not Null Test
Asserts that specified column(s) never contain null values.
```sql
-- @test_not_null: user_id, country
```

### 🔑 Unique Test
Asserts that specified column(s) contain strictly unique values.
```sql
-- @test_unique: user_id
```

### 🏷️ Accepted Values Test
Asserts that a column's values fall within a predefined list of allowed categories.
```sql
-- @test_accepted_values_segment: premium, free, enterprise
```

---

## 2. Relationship Tests (Foreign Key Validation)

Relationship tests validate referential integrity between tables. They ensure that every non-null value in a model's local column exists as a corresponding key in a parent dimension/mart model.

### 📝 Declaration Syntax
Specify the local column and the parent model/column in your model comments:
```sql
-- @test_relationship: <local_column> to <parent_model>.<parent_column>
```

### 💡 Example (`models/marts/customer_360.sql`)
```sql
-- @materialized: table
-- @test_relationship: user_id to stg_users.user_id

SELECT 
    u.user_id,
    u.country,
    COUNT(e.event_id) AS event_count
FROM stg_events e
LEFT JOIN stg_users u ON e.user_id = u.user_id
GROUP BY u.user_id, u.country;
```

### ⚙️ Behind the Scenes
At apply-time or test-time, SQLForge resolves the environment's schema for `stg_users` (e.g., `sqlforge__prod_staging.stg_users`) and executes the following validation query:
```sql
SELECT COUNT(*) 
FROM sqlforge__prod_marts.customer_360 
WHERE user_id IS NOT NULL 
  AND user_id NOT IN (SELECT user_id FROM sqlforge__prod_staging.stg_users);
```
If the count of orphan records is greater than `0`, the test fails.

---

## 3. Singular Tests (Custom SQL Assertions)

Singular tests are custom, standalone SQL queries stored inside a dedicated `tests/` directory at your project root. 

### 📐 Invariant Design
A singular test query represents a **failing condition**. 
* **Pass:** The query returns **0 rows** (i.e. no records violate your rule).
* **Fail:** The query returns **any rows** (the violating records are flagged).

### 💡 Example (`tests/assert_revenue_positive.sql`)
```sql
-- Singular Test: Daily revenue must never be negative
SELECT * 
FROM daily_metrics 
WHERE daily_revenue < 0;
```

### ⚙️ Dependency Resolution & Transpilation
Like models, singular tests are fully parsed. SQLForge automatically extracts their dependencies (e.g. `daily_metrics`) and replaces them with their fully qualified target tables in the current environment schema. It also transpiles any SQL dialects if you are executing against different warehouse backends.

---

## 4. Running Your Tests 🏃‍♂️

###stand-alone Test Command
Run all column validations, relationship checks, and singular assertions against a specific environment using `sqlforge test`:
```bash
sqlforge test prod
```

#### Output Report
SQLForge outputs a beautiful, color-coded execution report:
```
Running data quality tests for environment: prod

Running model column & relationship tests:
  stg_users.user_id (not_null) ....... PASS
  stg_users.country (not_null) ....... PASS
  stg_users.user_id (unique) ......... PASS
  stg_users.segment (accepted_values)  PASS
  customer_360.user_id (relationship) ... PASS (referenced stg_users.user_id)

Running custom singular assertion tests:
  assert_positive_prices ............ PASS

Test execution completed: 6 passed, 0 failed.
```
If any test fails, the command exits with code `1`, making it extremely easy to plug directly into GitHub Actions or CI/CD pipelines!

### In-Flight Materialization Tests
By default, model-level tests (Column and Relationship) run immediately after their respective tables are materialized during `sqlforge apply`. If any test fails, the pipeline is safely aborted to prevent dirty data from propagating downstream.
