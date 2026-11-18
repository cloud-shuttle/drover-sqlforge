---
title: "ADR 0008: Hosted SQLForge tenancy"
description: "How a future hosted SQLForge maps a customer org to a data project and isolated warehouse credentials."
product: drover-sqlforge
audience: platform-operator
doc_type: adr
topics:
  - tenancy
  - data-warehousing
  - governance-policy
  - security
surface: repo-docs
---

# 8. Hosted SQLForge tenancy

Date: 2026-11-18

## Status

Proposed. Design only. Not implemented in v1. This change adds no hosted endpoint, no tenancy code, no SpiceDB client, and no warehouse provisioning.

## Context

v1 SQLForge is one **data project** on one machine: `sqlforge.yml` names a **warehouse connection**, and every **environment** in that project shares it. `state.Manager.GetOrCreateEnv` sets the schema to `"sqlforge__" + name`. That prefix is `sqlident.envSchemaPrefix`. `ValidateEnvName` keeps `"sqlforge__" + name` inside `MaxIdentifierLen` (63). The CLI and the agent see that connection string because the operator put it in the workspace. [ADR 0002](0002-cli-invocation-drover-code-integration.md) defers hosted tenancy. [CONTEXT.md](../../CONTEXT.md) says v1 has no SQLForge tenancy model. [ADR 0007](0007-mcp-governance-gateway-identity-warden-audit.md) keeps authorization in Guard and Gateway: SQLForge does not grow a role matrix, and audit lines never contain SQL or secrets.

A future Drover Cloud deployment will run SQLForge for more than one customer. SQL execution is the isolation boundary. The mapping has to be fixed before any hosted endpoint exists. This ADR is that mapping. It does not build it.

Today's single-project path stays as it is. `sqlforge__<env>` is correct for one project with one warehouse login. It is not correct once two customers can both have an environment named `prod`.

## Decision

### 1. Customer org → data project → warehouse credentials

Three records, each owned by the platform, none of them taken from tool arguments:

| Record | Owner today | What hosted SQLForge uses |
|--------|-------------|---------------------------|
| **Customer org** | Drover Cloud. `provision.CustomerRecord` holds `CustomerID`, `Slug`, lifecycle status, and the platform-managed Git remote. `auth.TenantRootFromOrgID` maps the Zitadel org claim to a tenant root. | The org is the SpiceDB subject-scope and the billing scope. It is not a schema name. |
| **Data project** | Not a Cloud type in the reviewed checkout. In SQLForge it is one `sqlforge.yml` tree ([CONTEXT.md](../../CONTEXT.md)). | One customer org owns many data projects. Each project has its own environments. |
| **Warehouse credentials** | Not a Cloud or Gateway type. Gateway virtual keys (`x-bf-vk`) authorize LLM and MCP calls. Cloud's **job environment** injects an agent-facing or job-scoped virtual key, never a raw provider key. | Two secrets per data project, stored in the platform secret store. The tenant record holds secret ids only. |

Credentials are stored per tenant and never shared. Two data projects do not share a login, even inside one org. A login is never reused across orgs.

Reference is by indirection only:

- **Secret id.** The tenant record stores the secret-store id of the apply login and the id of the read-only login. It does not store the DSN.
- **Virtual-key-style handle.** The worker may hold an opaque handle that only the credential broker can exchange for a DSN. That handle is not a Gateway virtual key. `GATEWAY_API_KEY`, a standalone virtual key, and a job-scoped virtual key stay on the LLM/MCP hop. Using any of them as a warehouse credential is a defect.

SQLForge and the agent never see the raw DSN. Tool handlers, `sqlforge.yml`, MCP arguments, the job environment, and the audit log receive the tenant id, the data project id, the environment name, and a role class (`apply` or `read`). A Cloud credential broker is the only process that resolves the secret and opens the warehouse session. The single-project plugin channel `SQLFORGE_PLUGIN_DSN` is not the hosted path. This ADR does not add the broker.

`CustomerRecord.CustomerID`, `CustomerRecord.Slug`, and the UUID from `TenantRootFromOrgID` are not schema identifiers. UUIDs and hyphenated slugs fail `sqlident.ValidateIdentifier`. The hosted tenant record (a field that does not exist on `CustomerRecord` today) stores a separate immutable schema tenant id, assigned by the server when the project is bound.

### 2. SpiceDB relations

SQLForge does not grow a role matrix. Drover Guard is the authority, the same boundary as [ADR 0007](0007-mcp-governance-gateway-identity-warden-audit.md). Relation names below are the contract Guard enforces. They were not found as symbols in the Cloud or Gateway checkouts (see Reviewed against).

The customer-admin grants every relation. A platform operator does not grant them, and a member who is not a customer-admin does not grant them. Relations do not imply each other. A caller who plans and applies needs both `planner` and `applier`. A grant is on one object. The check requires the relation on the **data project** and on the **environment**. A project grant does not cover an environment that lacks it.

| Relation | Granted by | Allows | Warehouse login |
|----------|------------|--------|-----------------|
| `viewer` | customer-admin | `list_models`, `get_model`, `list_metrics`, `query_metric`, `list_snapshots` | Read-only role, and only if a session is opened. Compiling SQL does not open one. |
| `planner` | customer-admin | `plan_change` | None. `plan_change` does not open a warehouse session. |
| `applier` | customer-admin | `apply_change`, `run_snapshot` | Apply role. |

`get_model` stays on `viewer` for the Guard check. [ADR 0007](0007-mcp-governance-gateway-identity-warden-audit.md) still redacts model SQL for `Actor.Kind == "agent"` inside the Warden hook. Guard does not replace that redaction.

Missing relation, missing object, or an unknown tool name fails closed. SQLForge does not read `Actor.Roles` to allow a call. Those roles stay informational, as ADR 0007 already says.

### 3. Two warehouse roles per tenant

Each data project gets two warehouse roles, created with the project binding. Names come from the server-side schema tenant id:

- `sf_<tenant>_apply` — DDL and DML inside that tenant's schemas only. No superuser, no `CREATEROLE`, no `GRANT`, no other schema.
- `sf_<tenant>_read` — `SELECT` inside those schemas only. No insert, update, delete, create, or drop.

The apply role is not reused for preview, for `query_metric` execution, or for the model preview query. The tool class picks the login, not the caller. An `applier` who hits a preview path still gets `sf_<tenant>_read`. A `viewer` never gets `sf_<tenant>_apply`.

There is one apply role and one read role per tenant. There is no shared login across tenants.

### 4. Schema names

Single-project schemas stay `sqlforge__<env>`. Hosted schemas are:

```text
sf_<tenant>__<env>
```

`<tenant>` is the immutable schema tenant id from the server-side tenant record. It is a bare identifier under `sqlident.ValidateIdentifier`: ASCII letters, digits, and underscores, not starting with a digit, assigned by the server, never accepted from request input, MCP arguments, or `sqlforge.yml`. The server mints at most 16 characters so the full schema fits `MaxIdentifierLen` (63): `sf_` (3) + tenant (16) + `__` (2) leaves 42 characters for `<env>`. `<env>` still has to pass the `ValidateEnvName` character rule (start with a letter; letters, digits, underscores). A pair whose schema would exceed 63 bytes is rejected. Example: tenant `k7m2p9qx4n8b1c0d`, environment `prod` → `sf_k7m2p9qx4n8b1c0d__prod`.

`sf_` is a fixed prefix, same idea as `sqlforge__`. Callers cannot configure it.

The hosted database name is `sf_<tenant>`, also from the tenant record. A model annotation such as `-- @database:` may not select a different database. Apply rejects a database name that is not the tenant record's database. On a warehouse that cannot give each tenant its own database, the schema name plus the two roles are the boundary, and the database name is still taken from the tenant record.

Two customers with an environment named `prod` land in different schemas. The name is not a privilege boundary. The role grant is. The name stops accidental collisions when a statement is aimed at the right login.

### 5. Warden and Guard in front of apply

Order on a hosted MCP call:

1. **Gateway authenticates** the caller with a virtual key and forwards the identity token. The SQLForge client lists that header in `allowed_extra_headers` (`schemas.MCPClientConfig.AllowedExtraHeaders`). SQLForge verifies the token itself ([ADR 0007](0007-mcp-governance-gateway-identity-warden-audit.md)). Forwarded `X-User-ID` / `X-Org-ID` headers are not identity.
2. **Guard runs before the tool.** Gateway `GovernancePlugin.PreMCPHook` calls `governance.GuardClient.Evaluate` (`POST /v1/evaluate`). The hosted check is the SpiceDB relation on the data project and on the environment. `EvaluateRequest` in this Gateway checkout has one `ResourceID` (commented as the tool name) and one `Permission`. It cannot name both objects. Extending that payload is a Guard/Gateway change, not this repo, and it is not part of this ADR's change. Until Guard allows the relation, the tool does not run. Guard down or unreachable fails closed for a hosted call. The current client's "allow when Guard is not configured" branch is not acceptable for hosted SQLForge.
3. **Warden `scope: sqlforge` is the semantic hook.** It runs as the ADR 0007 `ToolHook` (`PreTool` / `PostTool`) and can only make the call stricter (`deny`, `redact`, `require_approval`). It cannot grant `viewer`, `planner`, or `applier`.
4. **Inside the tool**, after Guard and Warden have allowed the call: read-only MCP mode and protected environments from issue #55, and the SELECT-only rule from issue #53. Neither is implemented in this tree (`internal/mcp` has no `--read-only` flag and no `protected_environments`). When they land they still apply here. Read-only mode registers only read tools. A protected environment (default `prod`) refuses `apply_change` unless the process was started to allow it. A plan is bound to the environment that created it. `plan_change` still runs proposed SQL through `sqlguard.ValidateSelect`.

SQLForge does not self-approve. `require_approval` is an error returned to Guard or drover-code for HITL. The SQLForge process never turns that verdict into an allow.

### 6. Threat model

| Threat | Control | What it does not stop |
|--------|---------|------------------------|
| **Injection** | `plan_change` accepts one read-only statement: `sqlguard.ValidateSelect` (single `SELECT` / `WITH`, forbidden keywords and functions, five dialect lex modes, 256 KiB cap). Names SQLForge interpolates pass `ValidateIdentifier` / `ValidateEnvName`. Literals go through `QuoteLiteral`. Hosted tenant id and schema come from the tenant record, not from the request. | A legal `SELECT` that is expensive or reads data the role can already see. Git-committed model SQL is not passed through `sqlguard` (package comment in `internal/sqlguard`). A bug in the broker that executes a second statement the filter removed. |
| **Cross-tenant reads** | One apply role and one read role per tenant. No shared credentials. Privileges limited to `sf_<tenant>` / `sf_<tenant>__<env>`. Schema and database names come from the tenant record. Guard requires the relation on that data project and that environment before the tool runs. | A broker that attaches the wrong secret. A role granted too widely (the schema prefix is not a privilege). A fully qualified name of another schema inside Git SQL, if that role can see it. Identifier validation does not reject another tenant's well-formed name. |
| **Resource exhaustion** | Read and preview sessions: statement timeout 30 seconds. Result cap 50 rows on model preview (the same cap `internal/api` already puts in `LIMIT 50`) and 1 000 rows on any other result returned to a caller. The cap is on the session, not on the model remembering to write `LIMIT`. Apply sessions: statement timeout 15 minutes per statement, apply role only, so preview cannot take DDL locks with the apply login. One warehouse role pair per tenant, so one tenant has no shared login to burn. Gateway virtual-key budgets still limit the LLM/MCP hop only. | A legal apply that fills the tenant's own schema. Many statements that each finish under the timeout. CPU or disk contention on a shared warehouse cluster. Role isolation is not a resource group. Gateway rate limits do not meter warehouse compute. |
| **Credential leakage** | DSNs live in the secret store, one pair per tenant, referenced by secret id or by an opaque handle. SQLForge and the agent never receive the raw DSN. Audit ([ADR 0007](0007-mcp-governance-gateway-identity-warden-audit.md)) omits SQL text, literals, tokens, API keys, virtual keys, DSNs, and raw tool arguments. Errors are logged as a class (`auth`, `policy`, `warehouse`, `validation`), not as the driver message. Job environment injects Gateway virtual keys, not warehouse DSNs and not provider keys. | The broker process, which must hold the DSN in memory to open the session. An operator who can read the secret store. A warehouse error page that echoes the login name if the broker forwards the raw message despite the audit rule. Secret indirection does not help if a later change puts `SQLFORGE_PLUGIN_DSN` back into the agent environment. |

## Consequences

### Positive

- Two customers can both use `prod` without sharing a schema or a login.
- Guard remains the only place that answers "may this actor call this tool on this project and environment".
- Preview cannot use the login that is allowed to run DDL.
- The agent cannot exfiltrate a DSN that was never in its environment.

### Negative

- Hosted apply needs a credential broker that does not exist. Until that broker exists, hosted SQLForge must not be deployed.
- `EvaluateRequest` cannot yet name a data project and an environment. That is a Guard/Gateway change outside this repo.
- Customer ids and slugs cannot be copied into schema names. The schema tenant id is a new stored value.
- `ValidateEnvName`'s length budget assumes the `sqlforge__` prefix. Hosted names need their own budget (16 + 42). The single-project helper is not reused unchanged.

### Not in this change

- No hosted endpoint.
- No SpiceDB client, schema, or tuple writes in SQLForge.
- No warehouse provisioning, no secret-store client, no change to `GetOrCreateEnv` or `envSchemaPrefix`.
- No implementation of issue #55.
- v1 stays workspace-bound: one project, one warehouse connection, schema `sqlforge__<env>`.

## Reviewed against

Read on 2026-11-18 from the local checkouts, not from memory of a later commit.

**drover-cloud** (`/Users/peterhanssens/consulting/drover-org/drover-cloud`). Present. Branch `main` at `354de7a` (`feat: save chat replies on the company wiki`, 2026-11-16). The local `origin/main` ref is `03f6830`, and this checkout is **8 commits behind** it. Stale relative to that ref.

What is actually there:

- `CONTEXT.md` defines the platform customer, the Brain tenant record, and the **job environment**: `AGENT_TOKEN`, a Gateway virtual key (shared standalone key in Milestone A, job-scoped virtual key at GA), an ephemeral job credential for Brain MCP, and `x-bf-dim-*`. It states that workers do not receive raw provider keys. No warehouse DSN and no SQLForge data project.
- `internal/provision/types.go` `CustomerRecord`: `CustomerID`, `Slug`, `Plan`, `GitRepoURL`, lifecycle timestamps. No warehouse, secret id, or schema tenant field.
- `internal/provision/lifecycle_adapter.go` `tenantLifecycleAdapter`: Zitadel org, Git remote, Brain `POST /api/internal/tenants` (`internal/brain/client.go`). No warehouse provisioning.
- `internal/auth/orgid.go` `TenantRootFromOrgID`: Zitadel org claim → UUID tenant root. Output is not a bare SQL identifier.
- `internal/agentjobs/types.go` `CapabilityBindingSnapshot`: `AllowedMCPClientIDs`, `AllowedToolNames`, `AgentDefinitionRef`, `VersionHash`. `internal/gateway/client.go` `RegisterMCPAllowlist` posts that list. Neither type mentions a data project or a warehouse.
- `docs/explanation/hosted-governance-muster-guard-gateway.md`: Cloud registers a Guard binding, then Gateway `GovernancePlugin.PreMCPHook` calls Guard. The doc says Guard uses ReBAC and OPA. It does not name SpiceDB relations. `viewer`, `planner`, and `applier` are not in this checkout.
- `docs/explanation/control-plane-architecture.md`: hosted workers have no native provider API keys; LLM traffic uses the Gateway virtual key.

**drover-gateway** (`/Users/peterhanssens/consulting/drover-org/drover-gateway`). Present. Not on `main`. Branch `fix/vitest-4.1.11` at `28aeab54` (2026-11-17), tracking `origin/dependabot/npm_and_yarn/ui/vitest-4.1.11`. Local `origin/main` is `d6db9db`. Treat this as a feature-branch checkout, not as current `main`.

What is actually there:

- `CONTEXT.md`: a **virtual key** (`x-bf-vk`, `Authorization`, or `x-api-key`) is the caller credential. **Provider keys** stay on the gateway instance. A **governance customer** is not a platform customer unless provisioning links them. Milestone A uses standalone virtual keys; the job-scoped virtual key is GA. No warehouse credential.
- `core/schemas/mcp.go` `MCPClientConfig.AllowedExtraHeaders`: allow-list of headers forwarded to an MCP server at tool execution.
- `plugins/governance/main.go` `GovernancePlugin.PreMCPHook` and `plugins/governance/guard.go` `GuardClient.Evaluate` / `EvaluateRequest` / `ShouldCallGuard`. `ShouldCallGuard` calls Guard for hosted jobs when a job id dimension is present and the client is on the job's third-party list (or the fallback heuristic). `Evaluate` allows the call when `GUARD_URL` is empty (`guard-not-configured`). That branch is called out in §5 as not acceptable for hosted SQLForge.
- `docs/fork/capabilities.md` says the fork does not change Guard / SpiceDB / ReBAC boundaries. It is not a SpiceDB schema. No `viewer` / `planner` / `applier` symbols.

No SQLForge hosted endpoint, data-project type, or warehouse secret type was in either checkout. Where this ADR names a relation or a schema tenant id, that name is the contract for a later Guard and Cloud change, not a symbol found in those trees.
