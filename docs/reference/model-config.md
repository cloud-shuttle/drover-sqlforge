# Reference: SQL Model Configuration Comments

In SQLForge, model parameters, schemas, and data quality tests are declared directly inside the SQL files as configuration comments. This keeps the configuration colocated with the data logic and eliminates the need for separate YAML specification files for every model.

This document systematically lists and explains all supported model-level configuration comment keys.

---

## Declaration Syntax

All configuration comments must be declared with a double-dash prefix (`--`), followed by an `@` character, the parameter key, a colon (`:`), and the value.

```sql
-- @key_name: value_content
```

Any trailing comments using double-dashes (`--`) are ignored. Whitespace around keys and values is automatically trimmed.

### Complete Example (`models/marts/fct_orders.sql`)
```sql
-- @materialized: table
-- @schema: marts
-- @test_not_null: order_id, customer_id
-- @test_unique: order_id
-- @test_relationship: customer_id to stg_customers.customer_id

SELECT 
    order_id,
    customer_id,
    amount,
    created_at
FROM stg_orders;
```

---

## Key Reference

### `@materialized`
* **Type:** `string`
* **Required:** No
* **Default:** `view`
* **Description:** Defines the physical materialization type in your data warehouse.
* **Supported Values:**
  * `view` (Default): Compiles the model into a standard database view. Recommended for low-compute logical joins.
  * `table`: Compiles the model into a physical table. Replaced or rebuilt on every apply run where a change or upstream change is detected.
  * `incremental`: Creates a physical table on the initial run, and then merges or appends only new or updated records on subsequent runs.
  * `materialized_view`: Compiles the model into a database-native materialized view.
  * `kafka` / `nats` / `streaming`: Stream-processing engines. Generates streaming pipeline resources mapped through the warehouse runner.

---

### `@schema`
* **Type:** `string`
* **Required:** No
* **Default:** Directory name (relative to the `models/` root)
* **Description:** The target database schema where the compiled relation will be materialized.
* **Fallback Behavior:** If `@schema` is omitted:
  * A model at `models/staging/stg_users.sql` defaults to the `staging` schema.
  * A model at `models/marts/marketing/fct_campaigns.sql` defaults to the `marts_marketing` schema (path separators are replaced by underscores).
  * A model at `models/users.sql` defaults to your default environment schema.

---

### `@database`
* **Type:** `string`
* **Required:** No
* **Description:** Overrides the target database name where this model will be built. Helpful for cross-database data segregation.

---

## Data Quality Test Keys

These keys declare data assertions that run automatically immediately after DDL execution during `sqlforge apply` or standalone during `sqlforge test`.

### `@test_not_null`
* **Type:** `comma-separated list`
* **Description:** Asserts that specified column(s) never contain null values.
* **Example:**
  ```sql
  -- @test_not_null: user_id, email, created_at
  ```

---

### `@test_unique`
* **Type:** `comma-separated list`
* **Description:** Asserts that specified column(s) contain strictly unique values.
* **Example:**
  ```sql
  -- @test_unique: user_id, transaction_hash
  ```

---

### `@test_accepted_values_<column_name>`
* **Type:** `comma-separated list`
* **Description:** Dynamically asserts that the values of the column specified in the suffix (e.g. `_status`) fall strictly within the provided list of allowed values.
* **Examples:**
  ```sql
  -- Asserts that the "status" column only contains 'active', 'pending', or 'archived'
  -- @test_accepted_values_status: active, pending, archived

  -- Asserts that the "tier" column only contains 'free', 'pro', or 'enterprise'
  -- @test_accepted_values_tier: free, pro, enterprise
  ```

---

### `@test_relationship`
* **Type:** `string`
* **Formats:** 
  * `<local_column> to <parent_model>.<parent_column>`
  * `<local_column>-><parent_model>.<parent_column>`
* **Description:** Validates referential integrity (foreign key validation) between tables. Ensures that every non-null value in the local column has a matching value in the parent model's primary key column.
* **Behind the Scenes:** During environment execution, the parent model name is resolved to its fully qualified name inside the active schema environment.
* **Examples:**
  ```sql
  -- Standard 'to' syntax
  -- @test_relationship: customer_id to stg_customers.customer_id

  -- Clean arrow '->' syntax
  -- @test_relationship: product_id->stg_products.product_id
  ```
