package proxy_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

// BenchmarkHandlerWithPostgres measures the full proxy → govern → Postgres write
// latency. Target: p99 < 20ms on a local Postgres instance.
//
// Run with:
//
//	POSTGRES_TEST_DSN="postgres://auvis:secret@localhost:5432/auvis?sslmode=disable" \
//	  go test ./proxy/... -bench BenchmarkHandlerWithPostgres -benchtime=5s -count=3 -v
func BenchmarkHandlerWithPostgres(b *testing.B) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		b.Skip("POSTGRES_TEST_DSN not set — skipping Postgres benchmark")
	}

	// ── Fake upstream ─────────────────────────────────────────────────────────
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"user":{"name":"Jane Doe","ssn":"123-45-6789","email":"jane@example.com"}}`)
	}))
	defer upstream.Close()

	// ── proxy.Policy ────────────────────────────────────────────────────────────────
	policyYAML := `
- id: bench-pii-mask
  role: analyst
  rules:
    "user.ssn":
      action: mask
`
	policyDir := b.TempDir()
	if err := os.WriteFile(policyDir+"/bench.yaml", []byte(policyYAML), 0644); err != nil {
		b.Fatalf("write policy: %v", err)
	}
	store := policy.NewStore()
	if _, err := store.LoadDir(policyDir); err != nil {
		b.Fatalf("load policies: %v", err)
	}

	// ── Audit logger ──────────────────────────────────────────────────────────
	auditPath := b.TempDir() + "/audit.jsonl"
	auditLog, err := audit.NewRotatingLogger(auditPath, 512, 10, 7)
	if err != nil {
		b.Fatalf("audit logger: %v", err)
	}
	defer auditLog.Close()

	// ── Postgres destination ──────────────────────────────────────────────────
	tableName := fmt.Sprintf("auvis_bench_%d", time.Now().UnixNano())
	b.Logf("Using Postgres table: %s", tableName)

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
		b.Fatalf("connect postgres: %v", err)
	}
	defer func() {
		dropBenchTable(b, dsn, tableName)
		pgDest.Close()
	}()

	// ── Wire handler ──────────────────────────────────────────────────────────
	var requestCount int64
	cfg := config.Config{Port: "0", AuditFile: auditPath}
	transformFn := transform.NewStreamApplyFn(store)
	handler := proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, pgDest)

	// ── Benchmark loop ────────────────────────────────────────────────────────
	b.ResetTimer()
	b.ReportAllocs()

	var totalMs int64

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/users/bench", nil)
		req.Header.Set("X-User-ID", fmt.Sprintf("bench-user-%d", i))
		req.Header.Set("X-User-Role", "analyst")
		req.Header.Set("X-Auvis-Upstream", upstream.URL)

		rw := httptest.NewRecorder()

		start := time.Now()
		handler(rw, req)
		elapsed := time.Since(start)

		totalMs += elapsed.Milliseconds()

		if rw.Code != http.StatusOK {
			b.Fatalf("iteration %d: expected HTTP 200, got %d — body: %s", i, rw.Code, rw.Body.String())
		}
	}

	b.StopTimer()

	if b.N > 0 {
		avgMs := totalMs / int64(b.N)
		b.ReportMetric(float64(avgMs), "avg_ms/op")
		b.Logf("Average per-request latency (proxy+govern+postgres): %d ms", avgMs)
	}
}

// dropBenchTable removes the benchmark table from Postgres.
func dropBenchTable(b *testing.B, dsn, table string) {
	b.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		b.Logf("cleanup: connect failed: %v", err)
		return
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
		b.Logf("cleanup: drop table %s: %v", table, err)
	}
}
