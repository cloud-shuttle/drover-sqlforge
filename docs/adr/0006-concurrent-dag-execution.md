# 6. Concurrent DAG Execution Engine

Date: 2026-05-25

## Status

Accepted

## Context

In early versions of SQLForge (`v0.1.0-alpha`), models were applied sequentially. The execution engine would iterate over the `ChangedModels` and `Impacted` models one by one, issuing DDL statements to the target data warehouse. 

While this was robust and simple to debug, it led to unacceptable compilation and deployment times for large enterprise projects (e.g. data projects with 50+ independent staging models that parse raw data). Since data transformation workflows are fundamentally Directed Acyclic Graphs (DAGs), many models do not depend on one another and can be executed entirely in parallel.

## Decision

We have decided to replace the sequential execution loop with a highly concurrent, non-blocking worker pool architecture.

1. **In-Degree Scheduling:** Before execution begins, the engine maps out every changed and impacted model to calculate its *in-degree* (the number of upstream dependencies). Models with an in-degree of 0 are immediately ready for execution.
2. **Bounded Worker Pools:** The system spawns a configurable number of worker goroutines (defaulting to 4, overridable via `--threads`). These workers continuously pull from a shared `readyChan`.
3. **Event-Driven Orchestration:** As a worker finishes applying a model, it signals an orchestrator loop. The orchestrator uses a `sync.Mutex` to safely decrement the in-degree of all downstream dependents. If a dependent's in-degree reaches 0, it is pushed to the `readyChan`.
4. **Fast-Fail Contexts:** If any worker encounters a fatal error (e.g. a syntax error from the warehouse), the orchestrator triggers a `context.WithCancel()`. This gracefully halts all idle workers from picking up new jobs, preventing the DAG from entering an indeterminate state.

## Consequences

### Positive
- **Drastically Reduced Latency:** Wide DAGs will see compilation times cut by a factor roughly equal to the `--threads` limit.
- **Resource Saturation:** Better utilization of cloud data warehouse concurrency limits (e.g., Snowflake clusters).

### Negative
- **Debugging Complexity:** Logs and terminal outputs are now interleaved. It is more difficult to trace the exact chronological sequence of events without robust structured logging.
- **Connection Saturation:** If the `--threads` limit is set too high, it may overwhelm the target database with too many concurrent connections, leading to connection timeouts.
