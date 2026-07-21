package proxy_test

import (
	"encoding/json"
	"fmt"
	"io"
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
)

// TestE2EPineconeDestination verifies the full proxy → govern → Pinecone write path:
//  1. A fake upstream returns a JSON body containing an SSN (PII) and an embedding.
//  2. A policy masks the ssn field.
//  3. The governed response is written synchronously to Pinecone.
//  4. We query the Pinecone REST API to confirm the masked metadata and vector landed correctly.
//
// To run this test, set the environment variables:
//   PINECONE_TEST_API_KEY="your-api-key"
//   PINECONE_TEST_INDEX_HOST="your-index-abcd.svc.environment.pinecone.io"
//
// Command:
//   go test ./proxy/... -run TestE2EPineconeDestination -v
func TestE2EPineconeDestination(t *testing.T) {
	apiKey := os.Getenv("PINECONE_TEST_API_KEY")
	indexHost := os.Getenv("PINECONE_TEST_INDEX_HOST")
	indexHost = strings.TrimPrefix(indexHost, "https://")
	indexHost = strings.TrimPrefix(indexHost, "http://")

	if apiKey == "" || indexHost == "" {
		t.Skip("PINECONE_TEST_API_KEY or PINECONE_TEST_INDEX_HOST not set — skipping E2E Pinecone test")
	}

	// ── Fake upstream server ──────────────────────────────────────────────────
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Raw PII in the upstream response — the policy should mask the SSN.
		
		embedding := make([]float64, 1024)
		for i := 0; i < 1024; i++ {
			embedding[i] = 0.1
		}
		
		response := map[string]any{
			"user": map[string]string{
				"name": "Jane Doe",
				"ssn": "123-45-6789",
			},
			"embedding": embedding,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer upstream.Close()

	// ── Policy: mask ssn ─────────────────────────────────────────────────────
	policyYAML := `
policies:
  - name: test-pinecone-mask
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
	defer auditLog.Close()

	// ── Pinecone destination ──────────────────────────────────────────────────
	pineconeDest := destination.NewPineconeDestination(
		map[string]string{
			"index_host":   indexHost,
			"vector_field": "embedding",
		},
		pipelinecfg.AuthConfig{Password: apiKey},
		pipelinecfg.TLSConfig{},
	)
	if err := pineconeDest.Connect(); err != nil {
		t.Fatalf("connect pinecone destination: %v", err)
	}

	// ── Wire handler ──────────────────────────────────────────────────────────
	var requestCount int64
	cfg := config.Config{Port: "0", AuditFile: auditPath}
	transformFn := transform.NewStreamApplyFn(store)
	handler := proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, pineconeDest)

	// ── Fire a governed request ───────────────────────────────────────────────
	reqID := fmt.Sprintf("req-pinecone-test-%d", time.Now().UnixNano())
	
	req := httptest.NewRequest(http.MethodGet, "/api/vectors/1", nil)
	req.Header.Set("X-Request-ID", reqID)
	req.Header.Set("X-User-ID", "user-pinecone")
	req.Header.Set("X-User-Role", "analyst")
	req.Header.Set("X-Auvis-Upstream", upstream.URL)

	rw := httptest.NewRecorder()
	handler(rw, req)

	// ── Assert: client response ───────────────────────────────────────────────
	if rw.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d — body: %s", rw.Code, rw.Body.String())
	}
	
	respBody := rw.Body.String()
	if strings.Contains(respBody, "123-45-6789") {
		t.Fatal("FAIL: raw SSN found in client response — masking did not fire")
	}

	// Read request ID from audit log (proxy.Handler sets its own or keeps it)
	auditLog.Close() // Flush the async logger to disk before reading
	entries, err := readAuditLog(auditPath) // readAuditLog is defined in handler_integration_test.go
	if err != nil || len(entries) == 0 {
		t.Fatalf("failed to read audit log to find request ID: %v", err)
	}
	actualReqID := entries[len(entries)-1].RequestID
	if actualReqID == "" {
		t.Fatal("audit log did not contain a request_id")
	}

	// Wait a small moment to ensure Pinecone has processed the upsert
	time.Sleep(2 * time.Second)

	// ── Fetch from Pinecone to assert metadata ────────────────────────────────
	fetchURL := fmt.Sprintf("https://%s/vectors/fetch?ids=%s", indexHost, actualReqID)
	fetchReq, err := http.NewRequest(http.MethodGet, fetchURL, nil)
	if err != nil {
		t.Fatalf("create fetch request: %v", err)
	}
	fetchReq.Header.Set("Api-Key", apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(fetchReq)
	if err != nil {
		t.Fatalf("pinecone fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("fetch returned status %d: %s", resp.StatusCode, string(b))
	}

	var fetchResult struct {
		Vectors map[string]struct {
			ID       string         `json:"id"`
			Values   []float64      `json:"values"`
			Metadata map[string]any `json:"metadata"`
		} `json:"vectors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fetchResult); err != nil {
		t.Fatalf("decode fetch result: %v", err)
	}

	vector, ok := fetchResult.Vectors[actualReqID]
	if !ok {
		t.Fatalf("vector %q not found in Pinecone index", actualReqID)
	}

	// ── Assert Pinecone contents ──────────────────────────────────────────────
	if len(vector.Values) != 1024 {
		t.Errorf("expected 1024 embedding values, got %d", len(vector.Values))
	}

	ssnRaw, ok := vector.Metadata["user.ssn"]
	if !ok {
		t.Fatalf("user.ssn missing in pinecone metadata")
	}
	ssnStr, _ := ssnRaw.(string)
	
	if ssnStr == "123-45-6789" {
		t.Errorf("FAIL: raw SSN made it into pinecone metadata!")
	} else if ssnStr == "" {
		t.Errorf("FAIL: SSN was masked but is empty")
	} else {
		t.Logf("SUCCESS: SSN was masked in pinecone metadata as: %s", ssnStr)
	}
}
