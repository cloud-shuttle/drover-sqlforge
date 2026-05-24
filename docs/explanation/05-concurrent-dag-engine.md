# Understanding the Concurrent DAG Engine

SQLForge executes data models concurrently by analyzing the Directed Acyclic Graph (DAG) of your project. This document explains the internal mechanics of the concurrency model implemented in `internal/plan/apply.go`.

## The Problem with Sequential Execution

In a typical data project, you might have a wide layer of "staging" models that select directly from raw tables. Because these staging models don't depend on each other, executing them one by one is highly inefficient. 

```text
raw_users      raw_events      raw_subscriptions
    |              |                   |
stg_users      stg_events      stg_subscriptions
    \              |                   /
      \            |                 /
       \           |               /
         --- fact_user_activity ---
```
In a sequential engine, `stg_users`, `stg_events`, and `stg_subscriptions` run consecutively. In the concurrent engine, they run simultaneously.

## How it Works Under the Hood

The SQLForge execution engine uses a **Bounded Worker Pool** combined with **In-Degree Scheduling**.

### 1. In-Degree Calculation
Before any SQL is executed, the engine analyzes the `ExecutionPlan`. For every model, it calculates its **in-degree** (the count of upstream dependencies that must execute first). 
- In the example above, `stg_users` has an in-degree of `0`. 
- `fact_user_activity` has an in-degree of `3`.

### 2. The `readyChan` and Worker Pool
The engine creates a Go channel called `readyChan` and spawns a fixed number of worker goroutines (controlled by the `--threads` CLI flag). 
All models with an in-degree of `0` are immediately pushed into the `readyChan`. The workers immediately pick these up and begin sending DDL statements to the data warehouse.

### 3. Mutex Orchestration
When a worker finishes executing a model (e.g. `stg_users`), it sends a signal to a central orchestrator loop. 
The orchestrator locks a `sync.Mutex` and updates the state of the DAG:
1. It looks up all models that depend on `stg_users` (in this case, `fact_user_activity`).
2. It decrements the in-degree of `fact_user_activity` by 1.
3. Once `stg_events` and `stg_subscriptions` also finish, the in-degree of `fact_user_activity` drops to `0`.
4. The orchestrator immediately pushes `fact_user_activity` into the `readyChan`, and an available worker picks it up.

### 4. Fast-Fail Contexts
If any model execution fails (due to invalid SQL or a network timeout), the worker returns an error to the orchestrator. The orchestrator immediately calls `cancel()` on the global execution context.
This guarantees that:
- In-flight queries are gracefully handled or aborted (depending on the driver).
- Idle workers will not pick up any new jobs from the `readyChan`.
- The execution halts as quickly as possible to avoid corrupting the environment state.
