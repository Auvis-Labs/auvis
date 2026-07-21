package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Auvis-Labs/auvis/audit"
	"github.com/Auvis-Labs/auvis/config"
	"github.com/Auvis-Labs/auvis/pipeline/destination"
	"github.com/Auvis-Labs/auvis/policy"
	"github.com/Auvis-Labs/auvis/proxy"
	"github.com/Auvis-Labs/auvis/transform"

	pipelinecfg "github.com/Auvis-Labs/auvis/pipeline/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestE2EDestinationFanOut verifies the full proxy → govern → Postgres write path:
//  1. A fake upstream returns a JSON body containing an SSN (PII).
//  2. A policy masks the ssn field.
//  3. The governed response is written synchronously to Postgres.
//  4. We query Postgres to confirm the masked row landed correctly.
//  5. We confirm the audit entry carries destination_written: true.
//
// Set POSTGRES_TEST_DSN to run this test, e.g.:
//
//	POSTGRES_TEST_DSN="postgres://auvis:secret@localhost:5432/auvis?sslmode=disable" \
//	  go test ./proxy/... -run TestE2EDestinationFanOut -v
func TestE2EDestinationFanOut(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping E2E Postgres test")
	}

	// ── Fake upstream server ──────────────────────────────────────────────────
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Raw PII in the upstream response — the policy should mask the SSN.
		fmt.Fprintln(w, `{"user":{"name":"Jane Doe","ssn":"123-45-6789","email":"jane@example.com"}}`)
	}))
	defer upstream.Close()

	// ── Policy: mask ssn ─────────────────────────────────────────────────────
	policyYAML := `
policies:
  - name: test-pii-mask
    match:
      roles: [analyst]
      resource: /api/users/1
    rules:
      - field: "user.ssn"
        action: mask
`
	policyDir := t.TempDir()
	if err := os.WriteFile(policyDir+"/test.yaml", []byte(policyYAML), 0644); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	store := policy.NewStore()
	if _, err := store.LoadDir(policyDir); err != nil {
		t.Fatalf("load policies: %v", err)
	}

	// ── Audit logger ─────────────────────────────────────────────────────────
	auditPath := t.TempDir() + "/audit.jsonl"
	auditLog, err := audit.NewRotatingLogger(auditPath, 512, 10, 7)
	if err != nil {
		t.Fatalf("audit logger: %v", err)
	}

	// ── Postgres destination ──────────────────────────────────────────────────
	// Use a unique table per test run to avoid collisions.
	tableName := fmt.Sprintf("auvis_e2e_%d", time.Now().UnixNano())
	t.Logf("Using Postgres table: %s", tableName)

	pgDest := destination.NewPostgresDestination(
		map[string]string{
			"connection_string": dsn,
			"table":             tableName,
			"auto_create_table": "true",
		},
		pipelinecfg.AuthConfig{},
		pipelinecfg.TLSConfig{},
	)
	if err := pgDest.Connect(); err != nil {
		t.Fatalf("connect postgres destination: %v", err)
	}
	defer func() {
		// Drop the test table and close pool on cleanup.
		dropTable(t, dsn, tableName)
		pgDest.Close()
	}()

	// ── Wire handler ──────────────────────────────────────────────────────────
	var requestCount int64
	cfg := config.Config{Port: "0", AuditFile: auditPath}
	transformFn := transform.NewStreamApplyFn(store)
	handler := proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, pgDest)

	// ── Fire a governed request ───────────────────────────────────────────────
	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	req.Header.Set("X-User-ID", "user-42")
	req.Header.Set("X-User-Role", "analyst")
	req.Header.Set("X-Auvis-Upstream", upstream.URL)

	rw := httptest.NewRecorder()
	handler(rw, req)

	// ── Assert: client response ───────────────────────────────────────────────
	if rw.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d — body: %s", rw.Code, rw.Body.String())
	}
	responseBody := rw.Body.String()
	t.Logf("Governed client response: %s", responseBody)

	if strings.Contains(responseBody, "123-45-6789") {
		t.Error("FAIL: raw SSN '123-45-6789' found in client response — masking did not fire")
	}

	// ── Assert: Postgres row ──────────────────────────────────────────────────
	rowData, rowMeta, err := queryLatestRow(t, dsn, tableName)
	if err != nil {
		t.Fatalf("query Postgres row: %v", err)
	}
	t.Logf("Row data:     %s", rowData)
	t.Logf("Row metadata: %s", rowMeta)

	// Masked data must be stored — not the raw SSN.
	if strings.Contains(string(rowData), "123-45-6789") {
		t.Error("FAIL: raw SSN '123-45-6789' found in Postgres data column — governed body was not stored")
	}

	// Metadata must carry user context for auditability.
	var meta map[string]any
	if err := json.Unmarshal(rowMeta, &meta); err != nil {
		t.Fatalf("unmarshal row metadata: %v", err)
	}
	if got := fmt.Sprint(meta["user_id"]); got != "user-42" {
		t.Errorf("metadata.user_id: want user-42, got %q", got)
	}
	if got := fmt.Sprint(meta["user_role"]); got != "analyst" {
		t.Errorf("metadata.user_role: want analyst, got %q", got)
	}

	// ── Assert: audit log ─────────────────────────────────────────────────────
	auditLog.Close() // flush before reading
	entries, err := readAuditLog(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry, got none")
	}

	last := entries[len(entries)-1]
	if !last.DestinationWritten {
		t.Error("FAIL: audit entry destination_written should be true, got false")
	}
	t.Logf("Audit destination_ms: %d ms", last.DestinationMs)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// queryLatestRow fetches the most recently inserted (data, metadata) from the table.
func queryLatestRow(t *testing.T, dsn, table string) (data, metadata []byte, err error) {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	defer pool.Close()

	query := fmt.Sprintf(
		"SELECT data, metadata FROM %s ORDER BY timestamp DESC LIMIT 1",
		table,
	)
	row := pool.QueryRow(ctx, query)

	var d, m []byte
	if err := row.Scan(&d, &m); err != nil {
		return nil, nil, fmt.Errorf("scan row: %w", err)
	}
	return d, m, nil
}

// dropTable removes the test table from Postgres.
func dropTable(t *testing.T, dsn, table string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Logf("cleanup: could not connect to drop table %s: %v", table, err)
		return
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
		t.Logf("cleanup: drop table %s: %v", table, err)
	}
}

// readAuditLog parses the pretty-printed JSON blocks from the given audit log file.
func readAuditLog(path string) ([]audit.Entry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []audit.Entry
	for _, block := range strings.Split(strings.TrimSpace(string(raw)), "\n\n") {
		if block == "" {
			continue
		}
		var e audit.Entry
		if err := json.Unmarshal([]byte(block), &e); err != nil {
			return nil, fmt.Errorf("unmarshal audit line %q: %w", block, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
