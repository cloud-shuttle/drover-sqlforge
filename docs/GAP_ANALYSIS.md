# SQLForge Gap Analysis & Strategic Recommendations

## Executive Summary

Drover SQLForge represents a new generation of data transformation tools. By prioritizing compile-time AST analysis, Go-native performance, zero-copy environments, and autonomous agent capabilities (MCP), SQLForge occupies a unique "agentic-native" niche.

This document provides a comprehensive gap analysis of SQLForge compared to established tools like **dbt**, **SQLMesh**, and **Bruin**, followed by strategic recommendations on what features to prioritize next to accelerate adoption.

---

## 1. The SQLForge Advantage (What We Have)

These features represent our core differentiators and should be heavily marketed:

| Feature | SQLForge | dbt | SQLMesh | Bruin |
| :--- | :--- | :--- | :--- | :--- |
| **Model Parsing** | Pure SQL (WASM AST) | Jinja Templating | Jinja + SQLGlot AST | Jinja Templating |
| **Execution Speed** | Sub-second (Go) | Slow (Python) | Moderate (Python) | Sub-second (Go) |
| **Environments** | Zero-Copy (ClickHouse clones) | Physical Views/Tables | Virtual (Views/Pointers) | Physical |
| **Workflow** | Declarative (Plan & Apply) | Imperative (Run) | Declarative | Imperative |
| **Agentic / MCP** | **Native HTTP JSON-RPC** | None | None | None |
| **Semantic Layer** | Built-in (ANSI compiler) | MetricFlow (Abstracted) | Basic | None |
| **Dependencies** | Single Binary (No CGO) | Python environment | Python environment | Single Binary |

---

## 2. Gap Analysis (What We Are Missing)

We divide our feature gaps into two categories: intentional omissions that align with our philosophy, and unintentional gaps that we need to address to remain competitive.

### Intentional Omissions (By Design)

*   **Jinja Templating & Macros:**
    *   **The Gap:** Users cannot use `for`-loops to dynamically generate columns or rely on `{{ ref() }}` syntax.
    *   **Our Stance:** We reject Jinja to avoid "spaghetti code" and enforce pure, IDE-compatible SQL. Reusable logic should be pushed to the database layer via native User Defined Functions (UDFs).
*   **Python Runtime / External Dependencies:**
    *   **The Gap:** dbt and SQLMesh allow installation via `pip` and utilize Python-native drivers.
    *   **Our Stance:** We strictly maintain a single Go binary with no CGO bloat to ensure hyper-fast startup and portability, making it ideal for CI/CD and Unikraft deployments.

### Unintentional Gaps (Feature Parity Needs)

*   **SCD Type 2 History Tracking (Snapshots):**
    *   **The Gap:** Tracking slowly changing dimensions over time is a fundamental data warehousing need. dbt handles this natively via `dbt snapshot`. SQLForge currently lacks automated historical tracking.
*   **Explicit Column-Level Lineage:**
    *   **The Gap:** While our Polyglot WASM parser can map table dependencies, enterprise users increasingly expect rich, explicit column-level lineage reporting (as seen in dbt Cloud and SQLMesh).
*   **Ecosystem & Package Management:**
    *   **The Gap:** dbt's massive package hub (Fivetran ad attribution, Stripe data models) is a huge driver of adoption. SQLForge users currently have to build all business logic from scratch.
*   **Python/PySpark Models:**
    *   **The Gap:** For complex ML workloads or data science transformations where SQL falls short, dbt and SQLMesh allow Python models. SQLForge is currently restricted to SQL only.
*   **Built-in Schedulers / External Orchestration:**
    *   **The Gap:** Tools like SQLMesh can generate Airflow DAGs natively, and Bruin has basic scheduling. SQLForge relies entirely on the user's external orchestrator (e.g., GitHub Actions).

---

## 3. Strategic Recommendations (What We Should Build)

Based on the gap analysis, here is the prioritized roadmap for what SQLForge should build next to capture market share from dbt and SQLMesh.

### High Priority (Immediate Next Steps)

1.  **Implement Native Snapshots (SCD Type 2)**
    *   **Why:** This is a dealbreaker for many data teams migrating from dbt. 
    *   **How:** Build a `sqlforge snapshot` command that leverages our AST diffing engine to generate idempotent `MERGE` or `INSERT` statements that automatically handle `valid_from` and `valid_to` columns without requiring users to write the boilerplate.

2.  **Column-Level Lineage via Polyglot WASM**
    *   **Why:** We already have the hardest part built: the AST parser. Exposing column lineage is a massive enterprise feature that sets us apart from basic execution runners.
    *   **How:** Extend the Rust WASM module to output column tracking arrays alongside table dependencies. Expose this via the CLI (`sqlforge lineage`) and the MCP server so AI agents can query the lineage of a specific metric.

### Medium Priority (To Be Built Post-Alpha)

3.  **A Pure-SQL Package Management System**
    *   **Why:** To build community momentum, users need a way to share logic.
    *   **How:** Since we don't have Jinja, our packages won't be macros. Instead, we should allow users to import remote GitHub repositories containing pure SQL UDF definitions or `metrics.yml` structures that SQLForge can dynamically inject into the local DAG.

4.  **Airflow / Orchestrator Export Hooks**
    *   **Why:** Enterprise teams need to plug SQLForge into their existing enterprise schedulers.
    *   **How:** Create a command (e.g., `sqlforge export airflow`) that translates the internal SQLForge DAG execution plan into a native Python Airflow DAG script.

### Low Priority (Investigate or Ignore)

5.  **Python Models (via WASM)**
    *   **Recommendation:** **IGNORE FOR NOW.** Adding Python support compromises the single-binary, pure-SQL vision. If absolutely necessary in the future, investigate embedding a Python runtime via WASM (e.g., Pyodide), but do not natively integrate Python drivers.

6.  **Jinja Support**
    *   **Recommendation:** **NEVER BUILD.** Adding Jinja breaks the core philosophy of structural AST parsing and IDE-runnable SQL. We must stand firm on this differentiator.

---

## Conclusion

SQLForge's foundation is incredibly strong. By leaning into our AI-native architecture and speed, while selectively closing the gap on critical data warehousing features like Snapshots and Column-Level Lineage, we can position SQLForge as the modern, high-performance successor to dbt.
