# Contributing to AUVIS

Welcome! We are excited to have you contribute to **AUVIS (Intelligent Response Interception System)**. This document contains comprehensive, step-by-step guidelines for setting up your local development environment, running tests, understanding architecture components, and submitting contributions.

---

## 🏛️ System Architecture

AUVIS functions as a lightweight, policy-driven API gateway proxy. It sits in front of your upstream services, intercepts responses, and dynamically applies redaction, masking, and field exclusion based on user identity headers.

```
                    ┌────────────────────────┐
                    │      Client / UI       │
                    └───────────┬────────────┘
                                │ (Headers: X-User-ID, X-User-Role)
                                ▼
                    ┌────────────────────────┐
                    │      AUVIS Proxy       │ ◄─── Hot-reloaded YAML Policies
                    └───────────┬────────────┘
                                │ (Stripped Headers)
                                ▼
                    ┌────────────────────────┐
                    │  Upstream API Server   │ (e.g., Python FastAPI)
                    └────────────────────────┘
```

---

## 🛠️ Local Development Setup

To run AUVIS and its accompanying mock upstream server locally, ensure you have the following installed on your machine:
- **Go** (version `1.25.3` or later)
- **Python** (version `3.10` or later)
- **curl** (for manual testing)

---

### Step 1: Upstream Test Server Setup (`test-server`)

The `test-server` is a Python-based FastAPI mock application that serves rich PII datasets (e.g., transactions, user profiles, medical records) to simulate a real-world API.

1. Navigate to the `test-server` directory:
   ```bash
   cd test-server
   ```

2. Create a Python virtual environment to keep dependencies isolated:
   ```bash
   python3 -m venv .venv
   ```

3. Activate the virtual environment:
   * **macOS / Linux**:
     ```bash
     source .venv/bin/activate
     ```
   * **Windows**:
     ```bash
     .venv\Scripts\activate
     ```

4. Install the required Python packages:
   ```bash
   pip install --upgrade pip
   pip install -r requirements.txt
   ```

5. Launch the FastAPI test server:
   ```bash
   uvicorn main:app --port 3000 --reload
   ```
   The mock server will now be listening at **`http://localhost:3000`**. You can view the automated Swagger documentation at `http://localhost:3000/docs`.

---

### Step 2: Gateway Proxy Setup (`auvis-infra`)

The `auvis-infra` codebase is written in Go and contains the policy-matching engine, the reverse proxy, and the audit log writer.

1. In a new terminal window, navigate to the `auvis-infra` directory:
   ```bash
   cd auvis-infra
   ```

2. Configure the required environment variables:
   * **`AUVIS_UPSTREAM`** *(Required)*: The target URL of your backend API.
   * **`AUVIS_PORT`** *(Optional)*: The port the proxy will run on (Defaults to `8080`).
   * **`AUVIS_AUDIT_FILE`** *(Optional)*: Filepath for the append-only JSONL audit logs (Defaults to `./audit.jsonl`).

   Set these variables in your shell:
   ```bash
   export AUVIS_UPSTREAM=http://localhost:3000
   export AUVIS_PORT=8080
   export AUVIS_AUDIT_FILE=./audit.jsonl
   ```

3. Run the Go server:
   ```bash
   go run ./main.go
   ```
   AUVIS will start up, load files from the `./policies/` directory, and begin listening at **`http://localhost:8080`**.

> [!TIP]
> **Hot-Reloading in Action:**
> The Go proxy watches the `./policies/` directory using file-system notifications. You can modify your YAML policies live without restarting the proxy. Updates are loaded atomically and take effect instantly.

---

## 🧪 Manual Verification & Testing Guide

Once both servers are running, you can issue `curl` requests to the gateway port `8080` to verify policy enforcement.

Every request must include two header keys to identify the caller:
- `X-User-ID`: Custom string representing the user identity.
- `X-User-Role`: Determines which policy permissions to apply.

### Test Scenarios

#### 1. Full Admin Access (Wildcard Passthrough)
The admin role is authorized to view all raw data without any masking or field removal.
```bash
curl -i \
  -H "X-User-ID: admin-1" \
  -H "X-User-Role: admin" \
  http://localhost:8080/users
```

#### 2. Analyst Access (PII Redaction)
The analyst role is permitted to see the records but has sensitive fields like `email` masked and `phone` or `address` fully removed.
```bash
curl -i \
  -H "X-User-ID: analyst-1" \
  -H "X-User-Role: analyst" \
  http://localhost:8080/users
```

#### 3. Guest Access (Blocked Access)
The guest role is explicitly forbidden from accessing `/users`. A `403 Forbidden` response is returned.
```bash
curl -i \
  -H "X-User-ID: guest-1" \
  -H "X-User-Role: guest" \
  http://localhost:8080/users
```

#### 4. Default Deny-by-Design
Any unrecognized role (e.g., `hacker`) will be blocked by default unless a policy explicitly grants access.
```bash
curl -i \
  -H "X-User-ID: unauthorized-1" \
  -H "X-User-Role: hacker" \
  http://localhost:8080/users
```

---

## 🔍 Internal Management & Auditing Endpoints

AUVIS provides built-in endpoints to check system telemetry and inspect transaction history. These endpoints are registered before the catch-all proxy, ensuring they are never routed to the upstream service.

### 1. Telemetry Status (`/auvis/status`)
Retrieve information on uptime and total processed requests:
```bash
curl -s http://localhost:8080/auvis/status
```
*Example JSON output:*
```json
{
  "status": "ok",
  "uptime": "5m12s",
  "started_at": "2026-05-25T11:30:00Z",
  "requests_total": 42
}
```

### 2. Audit Trail (`/auvis/audit`)
Inspect the append-only log record database containing applied policies, duration telemetry, and masked fields:
```bash
curl -s "http://localhost:8080/auvis/audit?limit=5"
```
You can also filter the audit entries by query parameters:
* `limit` (Default: 50): Max entries to fetch.
* `role`: Filter by user role (e.g., `?role=analyst`).
* `user`: Filter by user ID.
* `resource`: Filter by matched prefix.

*Example filter request:*
```bash
curl -s "http://localhost:8080/auvis/audit?role=analyst&limit=10"
```

---

## ✍️ Contribution Workflow Guidelines

### Policy Definitions
- Policy configurations live as YAML files inside `/auvis-infra/policies/`.
- Ensure all rule actions match valid options: `allow`, `mask`, `remove`, `deny`.
- Before submitting rules, test your schema locally against valid API JSON outputs.

### Code Style Standards
- Follow standard Go formatting conventions (`go fmt ./...`).
- Run code checking and validation tools to verify static analysis passes cleanly:
  ```bash
  go vet ./...
  ```
- Keep Python code formatted using PEP 8 guidelines inside `test-server`.

### Branching and Pull Requests
1. Fork the repository and create your branch from `main`:
   ```bash
   git checkout -b feature/your-awesome-feature
   ```
2. Make your changes, keeping them focused and highly-tested.
3. Commit with descriptive messages following conventional commits standard:
   ```bash
   git commit -m "feat: implement response body compression support"
   ```
4. Verify the tests pass and push to your fork:
   ```bash
   git push origin feature/your-awesome-feature
   ```
5. Open a Pull Request detailing your changes and verification tests.
