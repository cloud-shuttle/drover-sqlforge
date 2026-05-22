# UI supply chain security

The SQLForge **Web GUI** (`ui/`) is the only npm surface in this repository. The shipped `sqlforge` binary embeds **static files** from `ui/dist/` (`ui/embed.go`); it does **not** run Node or load `node_modules` at runtime.

**Agent and CI paths** ([ADR 0002](../docs/adr/0002-cli-invocation-drover-code-integration.md)) use the Go CLI and MCP only—no npm on workers unless you explicitly build the GUI.

## Threat model

| Phase | Risk | Mitigation |
|-------|------|------------|
| `npm install` / `npm ci` on dev or CI | Malicious or compromised packages (install scripts, typosquatting) | Lockfile, `npm ci`, audit gate, review new **direct** deps |
| `npm run build` | Build-time code execution (Vite, plugins) | Trusted machines only; pin lockfile; optional `ignore-scripts` after lockfile review |
| **Runtime** (`sqlforge ui`) | Low—serves pre-built assets | No Node in production path; rebuild `dist/` from known lockfile for releases |

Recent registry **injection attacks** target install-time and maintainer compromise. Shrinking **direct** dependencies and avoiding custom rewrites of tiny transitive packages is more effective than leaving npm entirely for this stack.

## Dependency budget

**Direct dependencies (keep stable):**

| Kind | Packages | Notes |
|------|----------|--------|
| Runtime (7) | `react`, `react-dom`, `@xyflow/react`, `dagre`, `@types/dagre`, `lucide-react` | DAG viewer only; not required for `plan` / `apply` / MCP |
| Dev (13) | `vite`, `typescript`, `tailwindcss`, `eslint`, … | Build and lint only |

**Lockfile:** ~270 transitive packages—mostly Vite, ESLint, Tailwind, React tooling. Do **not** hand-audit each package; gate **direct** additions and automate audit.

### Adding a dependency

1. Prefer solving the problem without a new package (stdlib, small local code, or Go for product logic).
2. New **direct** dependency requires PR justification: maintainer activity, download footprint, install scripts, license.
3. Avoid packages with `postinstall` / `preinstall` unless reviewed; prefer `npm ci --ignore-scripts` in CI after the lockfile is trusted.
4. Run `npm audit` before merge; use `overrides` in `package.json` only for verified CVE fixes (see `brace-expansion`).
5. Bump with lockfile-only diffs—no manual edits to transitive versions without reason.

### Do not reimplement

Do **not** replace small transitive helpers (`brace-expansion`, `flatted`, etc.) in-tree. Do **not** rewrite `react`, `vite`, `dagre`, or graph layout—high cost, new security ownership, little reduction in registry exposure.

Revisit the **whole UI** (drop, separate repo, or minimal Go/HTML) before micro-forking npm packages.

## Required commands

From `ui/`:

```bash
npm ci          # reproducible install from package-lock.json
npm audit       # must report 0 vulnerabilities at moderate+ before merge
npm run build   # produces dist/ for go:embed
```

Root:

```bash
make ui         # npm ci + build (maintainers / GUI work)
make e2e        # full CLI + UI embed + e2e tests
```

For **CLI-only** work, you do not need npm unless you change `ui/` or run `sqlforge ui`. Go tests and `make e2e` still build the UI today because `go:embed` requires `ui/dist/`.

## Overrides and audit

- **`package.json` → `overrides`:** pin patched transitive versions when npm audit flags a CVE (e.g. `brace-expansion@5.0.6`).
- Re-run `npm install` and commit **both** `package.json` and `package-lock.json`.

## CI

Workflow: [`.github/workflows/ui-supply-chain.yml`](../.github/workflows/ui-supply-chain.yml)

Runs on PRs and `main` pushes that touch `ui/**`:

1. `npm ci`
2. `npm audit --audit-level=moderate`
3. `npm run build`
4. `go build ./cmd/sqlforge` (confirms `go:embed` of `dist/`)

Optional hardening: Socket.dev, `npm ci --ignore-scripts`, Dependabot/Renovate grouped updates.

## Alternatives (strategic)

| Approach | Supply-chain effect |
|----------|---------------------|
| **Status quo** (documented npm for `ui/` only) | Isolated; core product stays Go |
| **Optional UI build** (`make build-core` + committed or CI-built `dist/`) | Contributors skip npm unless touching GUI |
| **Remove in-binary UI** | Largest reduction; CLI + MCP remain |
| **Switch to pnpm/Bun** | Same registry risk; not a substitute for policy |

## Contacts

Security issues in SQLForge core: follow the org’s responsible disclosure process. For UI-only dependency concerns, include `package-lock.json` diff and `npm audit` output in the PR.

Note: Every PR touching the `ui/` directory is automatically analyzed by the UI supply chain CI workflow.

