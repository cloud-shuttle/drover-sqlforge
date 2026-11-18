---
title: "Reference: sqlforge.yml"
description: Fields the SQLForge loader reads from sqlforge.yml, including warehouse, AI, and MCP auth settings.
product: drover-sqlforge
audience: platform-operator
doc_type: reference
topics:
  - data-warehousing
  - mcp
  - identity-auth
surface: repo-docs
---

# Reference: `sqlforge.yml`

`sqlforge.yml` sits at the project root. `config.LoadConfig` reads it, expands `${VAR}` from the environment, and also loads a `.env` file in the same directory when one exists. Keys that are not listed here are ignored. `packages.yml` and `metrics.yml` are separate files.

```yaml
name: agentic_retail_2026
version: "1.0.0"
default_environment: prod

protected_environments:
  - prod

virtual:
  dialect: duckdb
  connection: memory

ai:
  provider: ollama
  model: llama3.2
  endpoint: http://localhost:11434

mcp:
  auth:
    mode: api_key
  hooks: []
```

## Top level

### `name`

String. Optional. The loader stores it and does not require it.

### `version`

String. Optional. The loader stores it and does not require it.

### `default_environment`

String. Optional. When set, `sqlforge env create` uses it as the base environment if `--base` is omitted, and a new runtime records it as the base of the environment it opens. When omitted, that base is `prod`.

This is not the environment a command runs against. `sqlforge plan`, `apply`, and `mcp` take the environment as an argument. `sqlforge mcp` with no argument uses `dev`.

### `protected_environments`

List of environment names. `apply_change` on the MCP server refuses these unless the process was started with `--allow-protected-apply`.

Omit the field and the protected set is `prod`. Set it to `[]` and nothing is protected. Empty strings in the list are skipped.

## `virtual`

The warehouse the runners open.

### `virtual.dialect`

String. Passed to `virtual.NewRunner`.

| Value | How it connects |
|---|---|
| `clickhouse`, or empty | In-process ClickHouse runner |
| `postgres` | In-process PostgreSQL runner (`lib/pq`) |
| anything else | gRPC plugin binary `sqlforge-plugin-<dialect>` |

Plugins shipped in this repo: `duckdb`, `snowflake`, `databricks`, `doris`, `velodb`. The core `sqlforge` binary does not link those drivers. DuckDB's plugin needs CGO. The process looks for the binary on `PATH`, then `./`, `../../`, and `../../bin/`. It receives the connection string as `SQLFORGE_PLUGIN_DSN`.

### `virtual.connection`

String. Dialect-specific DSN or path.

| Dialect | Example |
|---|---|
| DuckDB | `memory`, `:memory:`, or `stub` for an in-memory database. Any other string is a file path. |
| PostgreSQL | `postgres://user:password@localhost:5432/db?sslmode=disable` |
| ClickHouse | `clickhouse://default:password@localhost:9000/default` |

## `ai`

Used by `sqlforge ai explain`. When `sqlforge.yml` cannot be loaded, explain uses provider `ollama`, endpoint `http://localhost:11434`, and model `llama3.2`. A file that loads does not fill empty `ai` fields.

### `ai.provider`

`ollama` or `gateway`. Any other value, including an empty one, uses the Ollama client.

`gateway` posts an OpenAI-shaped chat completion to `{endpoint}/chat/completions`. The virtual key is read from `SQLFORGE_AI_VIRTUAL_KEY` and sent as `x-bf-vk`. There is no `virtual_key` field. A `virtual_key` entry in this file is ignored. The client refuses to dial when the environment variable is missing.

Gateway requests also copy `X_BF_DIM_*` environment variables to `x-bf-dim-*` headers. A value is sent only when it is 1–128 characters from `A-Z`, `a-z`, `0-9`, `_`, `.`, `:`, and `-`.

### `ai.model`

String. Model name sent to Ollama or Gateway.

### `ai.endpoint`

String. Base URL of the Ollama server or the Gateway.

## `mcp`

Governance for `sqlforge mcp`. The API key is not a field in this file. Set `SQLFORGE_MCP_API_KEY` or pass `--api-key`. The flag wins when both are set.

### `mcp.auth.mode`

`api_key` (the default when omitted) or `oidc`.

`api_key` checks `Authorization: Bearer`. With no key configured, the server runs only on a loopback bind. `oidc` reads the raw value of `mcp.auth.token_header`. It does not strip a `Bearer ` prefix and it does not fall back to the API key. It does not read `X-User-ID`, `X-Org-ID`, or `X-User-Roles`.

### `mcp.auth.issuer`

String. Required when mode is `oidc`. Must match the token `iss` exactly. Discovery loads `{issuer}/.well-known/openid-configuration` and then `jwks_uri`. The issuer URL must be `https`, or `http` on loopback.

### `mcp.auth.audience`

String. Required when mode is `oidc`. The token `aud` must contain this value.

### `mcp.auth.token_header`

String. Header that carries the OIDC token. Default `x-drover-identity-token`.

### `mcp.auth.required_org`

String. Optional. When set, the token must carry `urn:zitadel:iam:org:id` equal to this value.

Accepted signing algorithms are RS256 and ES256. `exp` and `nbf` allow 60 seconds of skew. `iat` is required.

### `mcp.hooks`

List of hook names. The server always installs the built-in redact hook, which drops `sql`, `ast_type`, and `ast_value` from `get_model` when the caller is an agent.

| Name | Effect |
|---|---|
| empty or `redact` | No extra hook. Redact is already installed. |
| `warden` | Startup fails. The public module does not import drover-warden. Both build tags fail closed. |
| anything else | Startup fails with `unknown mcp hook`. |
