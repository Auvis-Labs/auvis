# Auvis Quickstart Guide

> **Goal**: Go from zero to your first policy-governed, masked API response in under **5 minutes**.

Auvis sits as a policy enforcement gateway between clients and your upstream services. In this guide, you will run Auvis locally, configure a masking policy, and verify governed responses.

---

## 📋 Prerequisites

- **Go 1.22+** (for CLI/binary path) OR **Docker & Docker Compose** (for container path)
- `curl` or Postman for sending test HTTP requests

---

## 🚀 Option A: Go Install / Binary CLI Path (1 Minute)

### Step 1: Install Auvis CLI
```bash
# Option 1: Instant Go Install
go install github.com/cridiv/Auvis@latest

# Option 2: Build from Source
git clone https://github.com/cridiv/Auvis.git
cd Auvis
go build -o auvis .
```

### Step 2: Start the Mock Upstream Server
Auvis includes a built-in mock upstream server in `test-cmd` for testing:
```bash
# In a separate terminal tab:
cd test-cmd
go run main.go
# Starts mock upstream listening on http://localhost:8000
```

### Step 3: Define a Policy (`policies/analyst_mask.yaml`)
Create a YAML policy in `policies/analyst_mask.yaml`:
```yaml
policies:
  - name: analyst-mask-policy
    priority: 100
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: mask
      - field: "phone"
        action: mask
      - field: "ssn"
        action: remove
    compliance_tags: ["GDPR"]
```

### Step 4: Start Auvis Gateway
```bash
# Start Auvis listening on port 9090 pointing to mock upstream on port 8000
AUVIS_PORT=9090 ./auvis dev
```

### Step 5: Test Governed Output with `curl`

#### Send request with `analyst` role header:
```bash
curl -s -H "X-User-ID: usr_101" \
        -H "X-User-Role: analyst" \
        -H "X-Auvis-Upstream: http://localhost:8000" \
        http://localhost:9090/users | jq .
```

#### Expected Governed Output:
```json
[
  {
    "id": "usr_101",
    "name": "Jane Doe",
    "email": "***@example.com",
    "phone": "***-***-4567",
    "date_of_birth": "**-**-1990",
    "account_status": "active"
  }
]
```
Notice:
- `email`, `phone`, `date_of_birth` are automatically **masked**.
- `ssn` is completely **removed**.

---

## 🐳 Option B: Docker Container & Compose Path (2 Minutes)

### Step 1: Pull Image or Launch Compose
```bash
# Option 1: Direct Docker Pull
docker pull ghcr.io/cridiv/auvis:latest

# Option 2: Launch full stack via Compose
git clone https://github.com/cridiv/Auvis.git
cd Auvis
docker compose up -d
```
This starts:
- **Auvis Gateway** on port `8080`
- **PostgreSQL Database** on port `5432`

### Step 2: Check Gateway Health
```bash
curl -i http://localhost:8080/healthz
# Returns HTTP 200 OK
```

### Step 3: Test Governance
```bash
curl -s -H "X-User-ID: usr_101" \
        -H "X-User-Role: analyst" \
        http://localhost:8080/users | jq .
```

---

## 🔍 Step 6: Inspect Audit Trail

Every request passing through Auvis generates a structured audit log entry in `audit.jsonl`:

```json
{
  "timestamp": "2026-07-21T02:46:00Z",
  "request_id": "req-17845937576289",
  "user_id": "usr_101",
  "user_role": "analyst",
  "resource": "/users",
  "method": "GET",
  "upstream_status": 200,
  "summary": {
    "policies_applied": ["users-analyst-mask"],
    "fields_masked": ["email", "phone", "date_of_birth"],
    "fields_removed": ["ssn"]
  }
}
```

---

## 🛠️ Helpful CLI Commands

| Command | Description |
|---|---|
| `./auvis dev` | Start gateway in development hot-reload mode |
| `./auvis validate` | Validate policy YAML files for syntax and schema errors |
| `./auvis version` | Print binary version, Go version, and git commit |

---

## 📖 Next Steps

- Read **[Demo & Live Visualizer Guide](./demo.md)** for full details on the `auvisDemo` healthcare service, live SSE stream, draggable audit visualizer, and high-throughput concurrent benchmarks.
- Read **[Policy Reference Guide](./policies.md)** to learn about dynamic PII tags, fail-closed conditions, and custom regex validators.
- Read **[Architecture Overview](../README.md#architecture)** for deep-dive pipeline details.
