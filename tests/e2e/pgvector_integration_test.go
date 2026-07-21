package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Auvis-Labs/auvis/audit"
	"github.com/Auvis-Labs/auvis/config"
	pipelinecfg "github.com/Auvis-Labs/auvis/pipeline/config"
	"github.com/Auvis-Labs/auvis/pipeline/destination"
	"github.com/Auvis-Labs/auvis/policy"
	"github.com/Auvis-Labs/auvis/proxy"
	"github.com/Auvis-Labs/auvis/transform"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestE2EPgVectorDestination verifies the full proxy → govern → pgvector write path.
//
// Docker setup for pgvector testing:
//
//	docker run -d --name pgvector-e2e -p 5433:5432 \
//	  -e POSTGRES_USER=auvis -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=auvis \
//	  pgvector/pgvector:pg16
//
// Run command:
//
//	PGVECTOR_TEST_DSN="postgres://auvis:secret@localhost:5433/auvis?sslmode=disable" go test ./proxy/... -run TestE2EPgVectorDestination -v
func TestE2EPgVectorDestination(t *testing.T) {
	dsn := os.Getenv("PGVECTOR_TEST_DSN")
	if dsn == "" {
		t.Skip("PGVECTOR_TEST_DSN not set — skipping E2E pgvector test")
	}

	// ── Fake upstream server ──────────────────────────────────────────────────
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"user":{"name":"Jane Doe","ssn":"123-45-6789"},"embedding":[0.1,0.2,0.3]}`)
	}))
	defer upstream.Close()

	// ── Policy: mask ssn ─────────────────────────────────────────────────────
	policyYAML := `
policies:
  - name: test-pgvector-mask
    match:
      roles: [analyst]
      resource: /api/vectors/1
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

	// ── PgVector destination ──────────────────────────────────────────────────
	tableName := fmt.Sprintf("test_vectors_%d", time.Now().UnixNano()%100000)
	t.Logf("Using pgvector table: %s", tableName)

	dest := destination.NewPgVectorDestination(
		map[string]string{
			"connection_string": dsn,
			"table":             tableName,
			"vector_field":      "embedding",
			"vector_dim":        "3",
		},
		pipelinecfg.AuthConfig{},
		pipelinecfg.TLSConfig{},
	)
	if err := dest.Connect(); err != nil {
		t.Fatalf("connect pgvector destination: %v", err)
	}
	defer dest.Close()

	// ── Wire handler ──────────────────────────────────────────────────────────
	var requestCount int64
	cfg := config.Config{Port: "0", AuditFile: auditPath}
	transformFn := transform.NewStreamApplyFn(store)
	handler := proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, dest)

	// ── Fire a governed request ───────────────────────────────────────────────
	req := httptest.NewRequest(http.MethodGet, "/api/vectors/1", nil)
	req.Header.Set("X-User-ID", "user-pgvector")
	req.Header.Set("X-User-Role", "analyst")
	req.Header.Set("X-Auvis-Upstream", upstream.URL)

	rw := httptest.NewRecorder()
	handler(rw, req)

	// ── Assert: client response ───────────────────────────────────────────────
	if rw.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d — body: %s", rw.Code, rw.Body.String())
	}
	if strings.Contains(rw.Body.String(), "123-45-6789") {
		t.Fatal("FAIL: raw SSN found in client response — masking did not fire")
	}

	// ── Read request ID from audit log ────────────────────────────────────────
	auditLog.Close()
	entries, err := readAuditLog(auditPath) // defined in handler_integration_test.go
	if err != nil || len(entries) == 0 {
		t.Fatalf("failed to read audit log: %v", err)
	}
	actualReqID := entries[len(entries)-1].RequestID
	if actualReqID == "" {
		t.Fatal("audit log did not contain a request_id")
	}
	if !entries[len(entries)-1].DestinationWritten {
		t.Error("FAIL: audit entry destination_written should be true, got false")
	}

	// ── Verify data in PostgreSQL ─────────────────────────────────────────────
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect directly to postgres for verification: %v", err)
	}
	defer pool.Close()
	defer func() {
		// Cleanup the test table
		pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	}()

	var data, metadata, embeddingStr string
	query := fmt.Sprintf(`
		SELECT data::text, metadata::text, embedding::text
		FROM %s
		WHERE id = $1
	`, tableName)

	err = pool.QueryRow(context.Background(), query, actualReqID).Scan(&data, &metadata, &embeddingStr)
	if err != nil {
		t.Fatalf("failed to query pgvector row for id %q: %v", actualReqID, err)
	}

	// Assert data masking
	if strings.Contains(data, "123-45-6789") {
		t.Errorf("FAIL: raw SSN found in pgvector data column!")
	} else if !strings.Contains(data, "**-**-6789") && !strings.Contains(data, "MASKED") {
		// Depending on the exact mask implementation it might be different, but we check it's not the raw one
		t.Logf("SUCCESS: SSN was masked in pgvector data: %s", data)
	}

	// Assert vector extraction
	if strings.Contains(data, "embedding") {
		t.Errorf("FAIL: 'embedding' field was not stripped from data JSON! data=%s", data)
	}

	// Assert vector column
	if embeddingStr != "[0.1,0.2,0.3]" {
		t.Errorf("embedding column: want '[0.1,0.2,0.3]', got %q", embeddingStr)
	} else {
		t.Log("SUCCESS: embedding was written to pgvector column")
	}

	// Assert metadata
	if !strings.Contains(metadata, `"user_role":"analyst"`) {
		t.Errorf("metadata column missing user_role tag: %s", metadata)
	}
	if !strings.Contains(metadata, `"user_id":"user-pgvector"`) {
		t.Errorf("metadata column missing user_id tag: %s", metadata)
	}
}
