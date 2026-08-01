# ADR 001: Audit Logger Durability & Fsync Tradeoffs

**Status:** Accepted  
**Date:** 2026-07-30  
**Context:** Auvis Gateway v0.2.6 & v0.2.7  

---

## Context & Problem Statement

Auvis produces structured, newline-delimited JSON (`JSONL`) audit records for every governed API response. 

Initially (v0.1.0), the background audit writer invoked Go's `f.Sync()` (kernel `fsync`) after every log entry to guarantee zero data loss on hardware crashes. However, benchmarking under concurrent load (`hey -n 400000 -c 50`) revealed that per-entry `fsync` bottlenecked system throughput at **~1,710 req/sec** due to disk I/O serialization and kernel context switches.

When `fsync` was removed in v0.2.6, throughput increased **3.8× to 6,541 req/sec**. However, relying purely on OS buffer management meant that an ungraceful server power failure could cause loss of the most recent RAM-buffered log entries.

Different deployment environments require different tradeoffs:
- **High-throughput API Gateways:** Prioritize maximum request speed and sub-millisecond overhead.
- **Financial & Medical Compliance (PCI-DSS, HIPAA, SEC):** Require strict non-repudiation and zero log loss, accepting lower throughput.

---

## Decision

Auvis adopts **Configurable Fsync Durability Modes** with an explicit opt-in model:

1. **Default Mode (`audit_fsync_interval: "never"`):**
   - **Behavior:** Optimized for maximum throughput (~6,541 req/sec). Routine writes use standard OS file caching. Buffers are safely flushed on process termination (`Close()`).
   - **Tradeoff:** Potential loss of the last few milliseconds of logs in the event of an abrupt, ungraceful power failure.

2. **Synchronous Durability (`audit_fsync_interval: "always"` or `"every_entry"`):**
   - **Behavior:** Invokes `f.Sync()` synchronously after writing every single audit log entry.
   - **Tradeoff:** Throughput drops to ~1,700 req/sec due to disk I/O synchronization.
   - **Use Case:** Financial transactions, healthcare audit trails, and high-consequence admin endpoints requiring zero data loss guarantees.

3. **Dynamic Periodic Ticker (e.g. `audit_fsync_interval: "100ms"`, `"500ms"`, `"5s"`, `"1m"`):**
   - **Behavior:** Spawns a background `time.Ticker` at the user-specified Go duration. Disk buffers are flushed to physical storage periodically.
   - **Tradeoff:** Preserves high throughput (~6,400+ req/sec) while establishing a deterministic maximum data loss window (e.g. at most 100ms of logs lost on power failure).

4. **Explicit Startup Visibility:**
   - On gateway initialization, Auvis emits a prominent structured log line:
     `slog.Info("Audit logger initialized", "destination", l.path, "fsync_mode", l.fsyncInterval)`
   - This ensures it is never silently ambiguous which durability mode is running in production.

---

## Consequences

- Operators can select their exact speed vs. durability preference in `auvis.yaml` or via `AUVIS_AUDIT_FSYNC_INTERVAL`.
- The default behavior remains optimized for high throughput out of the box.
- Regulated deployments can opt into `"always"` or periodic intervals without custom code forks.
