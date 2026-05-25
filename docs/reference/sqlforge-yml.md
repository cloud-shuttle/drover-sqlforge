# Reference: `sqlforge.yml` Configuration

This document provides a comprehensive, systematic reference for all configuration options available in the `sqlforge.yml` project file.

The `sqlforge.yml` file is located at the root of your SQLForge project. It defines the workspace identity, default runtime behavior, warehouse connection details, and optional LLM provider integration.

---

## Configuration Schema

Below is a complete `sqlforge.yml` file displaying all supported configuration sections and keys:

```yaml
# The unique identifier for this data automation project
name: ecommerce_analytics

# The semantic version of your project schema
version: 1.0.0

# The default environment name used when no --env flag is supplied to the CLI
default_environment: dev

# Warehouse connection and dialect configuration
virtual:
  dialect: clickhouse
  connection: "clickhouse://default:password@localhost:9000/default?dial_timeout=200ms"

# Optional LLM integration configuration for agentic features (MCP)
ai:
  provider: anthropic
  model: claude-3-5-sonnet-latest
  endpoint: https://api.anthropic.com
```

---

## Key Reference

### `name`
* **Type:** `string`
* **Required:** Yes
* **Description:** The unique name of your data project. This identifier is used internally to namespace local state, environment schemas, and compile metadata.

### `version`
* **Type:** `string`
* **Required:** Yes
* **Description:** The semantic version of the project's schema (e.g. `1.0.0`). Useful for tracking schema changes over time.

### `default_environment`
* **Type:** `string`
* **Required:** No
* **Default:** `prod`
* **Description:** The target environment name to use if the `--env` parameter (or environment CLI argument) is omitted during `sqlforge plan`, `sqlforge apply`, or `sqlforge test`.

---

## `virtual` Section

The `virtual` configuration block defines the active connection to your analytics database or data warehouse. 

> [!NOTE]
> SQLForge uses a pure Go-native connection layer. Except for database-specific drivers compiled into external gRPC plugins, the core execution engine does not require CGO.

### `virtual.dialect`
* **Type:** `string`
* **Required:** Yes
* **Supported Values:**
  * `clickhouse` (native ClickHouse protocol)
  * `duckdb` (embedded/file-based analytics engine)
  * `postgres` (PostgreSQL and compatible warehouses)
  * `snowflake` (Snowflake Cloud Data Platform)
  * `doris` (Apache Doris real-time data warehouse)
  * `velodb` (VeloDB analytical store)
* **Description:** The SQL dialect and connection driver class.

### `virtual.connection`
* **Type:** `string`
* **Required:** Yes
* **Description:** The database-specific connection URI or file path. 
* **Examples by Dialect:**
  * **DuckDB:** `"/Users/user/data/my_project.db"` (a local file path)
  * **ClickHouse:** `"clickhouse://default:password@localhost:9000/default?dial_timeout=200ms"`
  * **PostgreSQL:** `"postgres://postgres:password@localhost:5432/my_db?sslmode=disable"`
  * **Snowflake:** `"user:password@account_identifier/db_name/schema_name?warehouse=wh_name"`

---

## `ai` Section

The optional `ai` configuration block provides parameters for the SQLForge Model Context Protocol (MCP) server. These settings instruct autonomous coding agents (like Cursor, Claude Desktop, or Drover Code) on which LLM provider to invoke when using agentic features.

### `ai.provider`
* **Type:** `string`
* **Required:** No
* **Supported Values:** `openai`, `anthropic`
* **Description:** The AI API provider for query synthesis, error recovery suggestions, and semantic compilation support.

### `ai.model`
* **Type:** `string`
* **Required:** No
* **Description:** The model name to call (e.g., `gpt-4o`, `claude-3-5-sonnet-latest`).

### `ai.endpoint`
* **Type:** `string`
* **Required:** No
* **Description:** The target URL of the LLM API. Allows routing queries through custom enterprise proxy Gateways, load balancers, or mock testing environments.
