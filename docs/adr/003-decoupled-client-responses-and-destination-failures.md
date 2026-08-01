# ADR 003: Decoupled Client HTTP Responses & Secondary Destination Write Failures

- **Status:** Accepted
- **Date:** July 29, 2026

## Context

Auvis supports secondary storage destination fan-out (Postgres, pgvector, Pinecone, Weaviate) to persist governed request payloads. Originally, destination writes operated synchronously in the proxy hot-path: if an external vector database (like Pinecone) encountered network latency, outages, or circuit breaker trips, Auvis aborted client request delivery and returned `HTTP 500 Internal Server Error`.

This design coupled primary client API availability to secondary database uptime.

## Decision

Auvis **decouples client HTTP response delivery from secondary storage destination write outcomes**:

1. **Governed Payload Delivery:** When a secondary storage write fails, panics, or fast-fails due to an open circuit breaker, Auvis logs a warning (`slog.Warn("Destination write failed — delivering governed response to client")`) and **delivers the governed HTTP 200 OK payload** to the client.
2. **Explicit Audit Log Tracking:** The failure is explicitly serialized in `audit.jsonl` with `"destination_written": false`, `destinations_failed: N`, and top-level `"destination_error": "..."` for post-incident compliance reconciliation.

## Consequences

- **High Availability:** Client API availability remains 100% operational even during total outages of secondary vector or SQL databases.
- **Full Compliance Queryability:** Compliance teams can filter `audit.jsonl` for `"destination_written": false` to identify unpersisted records.
