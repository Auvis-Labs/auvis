# Architecture Design Plan — Live Enterprise Streaming Demo System

This document outlines the end-to-end technical architecture, component design, and verification plan for demonstrating Auvis in a real-world enterprise streaming pipeline context (**Kafka → Auvis Engine → Postgres & pgvector + Grafana Observability**).

---

## 🎯 Demo Vision & Storyline

### The Scenario
A healthcare enterprise streams live patient records from their operational data bus (**Kafka topic: `patient-records`**) to populate a RAG vector index (**pgvector**) for an internal AI clinical assistant.

### The Problem
Raw operational events contain unmasked PII (Social Security Numbers, Emails, Phone Numbers, Credit Card/Billing Routing numbers, Patient IDs). If ingested directly into the vector database, any employee querying the AI tool can retrieve raw PII.

### The Auvis Solution
Auvis sits between Kafka and the vector index as an automated governance checkpoint:
1. **Consumes** raw patient records from Kafka via `KafkaSource`.
2. **Evaluates Policies** in $<0.01\text{ ms}$ in-memory (`policies/patient_demo.yaml`).
3. **Transforms Payload**: Masks emails/phones/billing IDs, deletes SSNs, and applies vaultless `pseudonymize` to patient IDs.
4. **Writes Governed Output** synchronously into **PostgreSQL** and **pgvector**.
5. **Emits Metrics** to Prometheus/Grafana in real-time.

---

## 🏗️ System Architecture & Data Flow

```
                      +------------------------------------------+
                      |  Synthea Patient Dataset Generator       |
                      +--------------------+---------------------+
                                           |
                                           v
                             High-Speed Stream Producer
                             (scripts/demo_producer.go)
                             (--rate=200/s --total=10,000)
                                           |
                                           v
                             +----------------------------+
                             |   Kafka Broker (Docker)    |
                             |   Topic: "patient-records" |
                             +-------------+--------------+
                                           |
                                           v
                             +----------------------------+
                             |   KafkaSource Connector    |
                             +-------------+--------------+
                                           |
                                           v
                             +----------------------------+
                             |   AUVIS PIPELINE ENGINE    |
                             |  - Policy: patient_demo    |
                             |  - Mask / Pseudonymize     |
                             |  - Circuit Breakers        |
                             |  - Fsync Audit Logging     |
                             +------+--------------+------+
                                    |              |
                    Governed SQL    |              |   Governed Vector
                     Records        v              v     Embeddings
                             +────────────+   +────────────────+
                             | PostgreSQL |   |    pgvector    |
                             +────────────+   +────────────────+
                                    ▲              ▲
                                    └──────┬───────┘
                                           |
                            RAG Search Verification Script
                             (scripts/demo_rag_query.go)
```

---

## 📂 Proposed File Structure & Components

### 1. Docker Orchestration Stack (`examples/docker-compose.demo.yml`)
Spawns the local demonstration infrastructure:
* **Kafka Broker (KRaft Mode):** Listening on port `9092`.
* **PostgreSQL + pgvector:** Listening on port `5432` with pre-initialized table `governed_patients`.
* **Prometheus:** Scrapes Auvis telemetry on port `9090`.
* **Grafana:** Dashboard listening on port `3000` with pre-loaded `examples/grafana_dashboard.json`.

### 2. High-Speed Kafka Stream Producer (`scripts/demo_producer.go` [NEW])
* Pure-Go generator streaming synthetic healthcare records into Kafka topic `patient-records`.
* Flags:
  * `--rate N`: Records per second (e.g. `200`).
  * `--total N`: Total records to stream (e.g. `10,000`).
  * `--brokers`: Kafka brokers (default `localhost:9092`).

### 3. Healthcare Policy Definition (`examples/policies/patient_demo.yaml` [NEW])
```yaml
policies:
  - name: patient-governance-policy
    priority: 100
    match:
      roles: ["*"]
      resource: "patient-records"
    rules:
      - field: "email"
        action: mask                      # alice@example.com -> ***@example.com
      - field: "ssn"
        action: remove                    # Deleted entirely from output payload
      - field: "phone"
        action: mask                      # +1-555-123-4567 -> ***-***-4567
      - field: "billing_account"
        action: mask                      # 123456789 -> *****6789
      - field: "patient_id"
        action: pseudonymize              # PT-101 -> usr_7f8a92b1 (format-preserving HMAC)
      - field: "medical_notes"
        action: allow                     # Passthrough clinical text for vector search
    compliance_tags: ["HIPAA", "GDPR"]
```

### 4. Pipeline Config (`examples/pipeline_demo.yaml` [NEW])
```yaml
pipeline:
  name: "patient-ingestion-pipeline"
  source:
    type: "kafka"
    settings:
      brokers: "localhost:9092"
      topic: "patient-records"
      group_id: "auvis-governance-group"
  policies_dir: "./examples/policies"
  destinations:
    - type: "postgres"
      settings:
        host: "localhost"
        port: "5432"
        dbname: "auvis_demo"
        table: "governed_patients"
        auto_create_table: "true"
    - type: "pgvector"
      settings:
        host: "localhost"
        port: "5432"
        dbname: "auvis_demo"
        table: "patient_vector_index"
        vector_field: "embedding"
        vector_dim: "1536"
```

### 5. RAG Search Verification Script (`scripts/demo_rag_query.go` [NEW])
* Connects to `pgvector` after ingestion.
* Executes semantic similarity search: *"Find clinical records related to Hypertension"*.
* Prints side-by-side verification showing **100% relevant medical match results** while confirming **0% raw PII (SSNs, raw emails) exists in the database**.

---

## 🔍 Verification Plan

### Automated & Manual Verification Steps

1. **Infra Health Check:** Launch `docker-compose.demo.yml` and verify Kafka, Postgres, Prometheus, and Grafana are healthy.
2. **Stream Execution:** Run `go run scripts/demo_producer.go --rate 200 --total 5000`.
3. **Pipeline Ingestion:** Start Auvis pipeline daemon listening on Kafka topic `patient-records`.
4. **Live Grafana Telemetry:** Observe `auvis_destination_writes_total` and `auvis_request_duration_seconds` updating live on Grafana (`http://localhost:3000`).
5. **RAG Proof:** Run `go run scripts/demo_rag_query.go` to prove vector search accuracy without PII leakage.
