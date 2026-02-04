# Internal Monitoring & Observability System

## 1. Goal of This Project

This document describes the design of an internal monitoring and observability
system built specifically for our backend architecture and operational needs.

The goal is not to replace every existing monitoring tool, but to:
- Gain deep understanding of our own system behavior
- Reduce debugging time during incidents
- Avoid excessive configuration overhead and tool sprawl
- Enable targeted, controlled investigation of production issues

This document is intentionally technical and internal.

---

## 2. Core Assumptions

- The backend will grow in size and complexity
- Failures will be partial and non-obvious
- Monitoring systems themselves can fail
- Engineers will debug systems under stress
- Not all problems can be solved with dashboards alone

---

## 3. High-Level Architecture

The monitoring system is split into four layers with strict responsibilities:

1. Signal Plane (data generation)
2. Transport & Aggregation Plane
3. Analysis & Storage Plane
4. Control & Query Plane

Each layer must be independently understandable and replaceable.

---

## 4. Signal Plane (Data Generation)

### 4.1 Service Instrumentation

Business services may include a lightweight monitoring dependency that:
- Measures request count (RPS)
- Measures request and function execution duration
- Emits errors and failure reasons
- Propagates trace context

This instrumentation runs inside the service process and has access to
business-level context.

Limitations:
- Cannot observe infrastructure internals (DB state, DNS, load balancers)
- Cannot detect failures before requests reach the service
- Adds minimal overhead to application code

---

### 4.2 External Observers (Agents / Watch Towers)

Some components must be monitored externally because they are not part of
business services:

- Databases
- Message queues
- DNS
- Load balancers
- Network paths
- Historically unstable components

For these cases, long-running observer processes ("Watch Towers") are used.

Watch Towers:
- Run outside business services
- Collect state and health information
- Maintain local baselines
- Emit structured signals instead of raw metrics
- Are scoped to specific components or problem areas

Watch Towers are always-on and low frequency by default.

---

### 4.3 Edge Measurement (Reverse Proxy / Ingress)

System entry points (reverse proxies, ingress controllers, gateways) are used to:
- Measure total incoming request rate
- Measure response latency
- Capture status codes
- Detect traffic drops or spikes

This provides full coverage for traffic that never reaches application code.

---

## 5. Transport & Aggregation Plane

### Responsibility

Move observability data from producers to storage and analysis systems.

### Characteristics

- Push-based where possible
- Backpressure-aware
- No business logic
- No alerting
- No long-term state

This layer must remain simple and reliable.

---

## 6. Analysis & Storage Plane

### Responsibility

Store and evaluate collected data.

This includes:
- Time series metrics
- Traces
- Logs (optional / partial)
- Watch Tower outputs

### Constraints

- No direct control over production systems
- Must tolerate partial data loss
- Must surface gaps and blind spots explicitly

Alert evaluation may live here, but alerts must always include context.

---

## 7. Control & Query Plane

This is the primary interaction layer for engineers.

It exposes a unified interface for:
- Querying metrics, traces, logs, and state
- Correlating signals across systems
- Performing controlled debugging actions

This layer is accessed by both CLI and UI.

---

## 8. Query Gateway

### Purpose

Provide a single entry point for querying observability data and control actions.

The Query Gateway:
- Authenticates requests
- Enforces permissions and limits
- Routes requests to the correct backend services
- Normalizes responses into stable schemas
- Supports partial responses

The gateway does not own data and does not perform heavy computation.

---

### 8.1 Backend Services Behind the Gateway

Examples:
- Trace service (knows everything about traces)
- Metrics service / TSDB
- Log service
- Deployment history service
- Watch Tower registry

Each backend is responsible for its own domain.
The gateway only coordinates access.

---

## 9. CLI

### Role

The CLI is the primary tool for engineers.

It:
- Queries the Query Gateway
- Formats and filters results
- Supports scripting and automation
- Works even when the UI is unavailable

The CLI contains no business logic.
It is a reference client for the control plane.

---

## 10. UI

### Role

The UI is a visualization and coordination layer.

It:
- Uses the same APIs as the CLI
- Visualizes query results
- Shows timelines, correlations, and history
- Adds friction and previews for risky actions

The UI does not have special powers.

---

## 11. Debug Drones (Ephemeral Debugging)

Debug Drones are short-lived diagnostic agents deployed on demand.

Characteristics:
- Explicitly triggered
- Time-limited (TTL enforced)
- Scoped permissions
- Fully audited

Use cases:
- Deep inspection of live systems
- Temporary high-cardinality metrics
- Profiling or packet inspection
- One-off debugging sessions

Debug Drones are never always-on.

---

## 12. Watch Towers (Persistent Observers)

Watch Towers are long-running observers attached to specific components.

They:
- Monitor known problem areas
- Maintain historical baselines
- Detect recurring patterns
- Escalate anomalies with context

Watch Towers do not mutate systems.

---

## 13. Measuring Throughput and Latency

### Request Rate (RPS)

Measured at:
- Reverse proxy / ingress (full coverage)
- Service instrumentation (context-rich)

### Latency

Measured via:
- Service instrumentation (processing time)
- Reverse proxy metrics (end-to-end time)
- Distributed tracing (cross-service latency)

Combining all three provides accurate system behavior.

---

## 14. Failure Handling

The monitoring system must:
- Detect its own partial failures
- Return partial results instead of failing entirely
- Clearly mark missing or stale data

Loss of observability is treated as an incident.

---

## 15. Security and Audit

- All control actions are logged
- Elevated access is time-bound
- Every action is attributable to a user or system

Debugging power is treated as a privileged capability.

---

## 16. Non-Goals

- This system is not a deployment platform
- This system is not a configuration manager
- This system is not a replacement for application correctness
- This system does not aim to compete feature-for-feature with existing tools

---

## 17. Open Questions / Next Steps

- What minimal storage backend is sufficient initially?
- How much sampling is acceptable for traces?
- How are Watch Towers deployed and owned?
- How much automation is allowed before human approval?
- When does it make sense to integrate external tools instead of replacing them?

This document is expected to evolve as the system is prototyped and used.
