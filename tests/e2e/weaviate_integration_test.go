package e2e_test

import (
	"crypto/sha1"
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

// TestE2EWeaviateDestination verifies the full proxy → govern → Weaviate write path:
//  1. A fake upstream returns a JSON body containing an SSN (PII) and an embedding.
//  2. A policy masks the ssn field.
//  3. The governed response is written synchronously to Weaviate.
//  4. We fetch the object back via GET /v1/objects/{class}/{uuid} and verify:
//     - The SSN is masked in properties (not raw "123-45-6789").
//     - The user_role ACL tag is stored in properties.
//     - The vector dimension is correct.
//
// Docker setup (anonymous access — no API key needed for local testing):
//
//	docker run -d --name weaviate-e2e -p 8080:8080 \
//	  -e AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true \
//	  semitechnologies/weaviate:1.25.1
//
// Run (anonymous Docker):
//
//	WEAVIATE_TEST_HOST="http://localhost:8080" go test ./proxy/... -run TestE2EWeaviateDestination -v
//
// Run (Weaviate Cloud):
//
//	WEAVIATE_TEST_HOST="https://my-cluster.weaviate.network" \
//	WEAVIATE_TEST_API_KEY="your-api-key" \
//	go test ./proxy/... -run TestE2EWeaviateDestination -v
func TestE2EWeaviateDestination(t *testing.T) {
	host := os.Getenv("WEAVIATE_TEST_HOST")
	if host == "" {
		t.Skip("WEAVIATE_TEST_HOST not set — skipping E2E Weaviate test")
	}
	apiKey := os.Getenv("WEAVIATE_TEST_API_KEY") // optional for anonymous Docker

	// ── Fake upstream server ──────────────────────────────────────────────────
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"user":{"name":"Jane Doe","ssn":"123-45-6789"},"embedding":[0.1,0.2,0.3]}`)
	}))
	defer upstream.Close()

	// ── Policy: mask ssn ─────────────────────────────────────────────────────
	policyYAML := `
policies:
  - name: test-weaviate-mask
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

	// ── Weaviate destination ──────────────────────────────────────────────────
	// Use a unique class per test run to avoid cross-run interference.
	className := fmt.Sprintf("AuvisE2E%d", time.Now().UnixNano()%1_000_000_000)
	t.Logf("Using Weaviate class: %s", className)

	wvDest := destination.NewWeaviateDestination(
		map[string]string{
			"host":              host,
			"class":             className,
			"vector_field":      "embedding",
			"auto_create_class": "true",
		},
		pipelinecfg.AuthConfig{Password: apiKey},
		pipelinecfg.TLSConfig{},
	)
	if err := wvDest.Connect(); err != nil {
		t.Fatalf("connect weaviate destination: %v", err)
	}
	defer deleteWeaviateClass(t, host, apiKey, className) // cleanup class after test

	// ── Wire handler ──────────────────────────────────────────────────────────
	var requestCount int64
	cfg := config.Config{Port: "0", AuditFile: auditPath}
	transformFn := transform.NewStreamApplyFn(store)
	handler := proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, wvDest)

	// ── Fire a governed request ───────────────────────────────────────────────
	req := httptest.NewRequest(http.MethodGet, "/api/vectors/1", nil)
	req.Header.Set("X-User-ID", "user-weaviate")
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
	last := entries[len(entries)-1]
	if !last.DestinationWritten {
		t.Error("FAIL: audit entry destination_written should be true, got false")
	}
	t.Logf("Audit destination_ms: %d ms", last.DestinationMs)

	// Weaviate may need a short moment to make the new object queryable.
	time.Sleep(1 * time.Second)

	// ── Compute the UUID WeaviateDestination stored this object under ─────────
	// The production code calls deterministicUUID() for non-UUID IDs.
	// We replicate that logic here so we can predict the fetch URL.
	objUUID := testDeterministicUUID(last.RequestID)
	t.Logf("Fetching Weaviate object: class=%s id=%s", className, objUUID)

	// ── Fetch the object from Weaviate ────────────────────────────────────────
	fetchURL := fmt.Sprintf("%s/v1/objects/%s/%s?include=vector", host, className, objUUID)
	fetchReq, _ := http.NewRequest(http.MethodGet, fetchURL, nil)
	if apiKey != "" {
		fetchReq.Header.Set("X-Weaviate-Api-Key", apiKey)
		fetchReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(fetchReq)
	if err != nil {
		t.Fatalf("weaviate fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("fetch returned status %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		ID         string         `json:"id"`
		Class      string         `json:"class"`
		Properties map[string]any `json:"properties"`
		Vector     []float64      `json:"vector"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode fetch result: %v", err)
	}

	// ── Assert Weaviate object contents ───────────────────────────────────────
	if result.Class != className {
		t.Errorf("class: want %q, got %q", className, result.Class)
	}

	if len(result.Vector) != 3 {
		t.Errorf("vector: want 3 dims, got %d", len(result.Vector))
	}

	ssnRaw := fmt.Sprint(result.Properties["user.ssn"])
	if ssnRaw == "123-45-6789" {
		t.Errorf("FAIL: raw SSN made it into Weaviate properties!")
	} else if ssnRaw == "" || ssnRaw == "<nil>" {
		t.Errorf("FAIL: user.ssn missing from Weaviate properties")
	} else {
		t.Logf("SUCCESS: SSN was masked in Weaviate properties as: %s", ssnRaw)
	}

	if got := fmt.Sprint(result.Properties["user_role"]); got != "analyst" {
		t.Errorf("user_role: want analyst, got %q", got)
	}
	if got := fmt.Sprint(result.Properties["user_id"]); got != "user-weaviate" {
		t.Errorf("user_id: want user-weaviate, got %q", got)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// testDeterministicUUID mirrors the deterministicUUID logic inside WeaviateDestination
// so the test can predict what UUID an arbitrary record ID was stored under in Weaviate.
// This keeps the test self-contained without importing internal package symbols.
func testDeterministicUUID(name string) string {
	namespace := [16]byte{
		0x6b, 0xa7, 0xb8, 0x11,
		0x9d, 0xad, 0x11, 0xd1,
		0x80, 0xb4, 0x00, 0xc0,
		0x4f, 0xd4, 0x30, 0xc8,
	}

	// Check if already a valid UUID
	if len(name) == 36 {
		valid := true
		for i, c := range name {
			if i == 8 || i == 13 || i == 18 || i == 23 {
				if c != '-' {
					valid = false
					break
				}
			} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				valid = false
				break
			}
		}
		if valid {
			return strings.ToLower(name)
		}
	}

	h := sha1.New()
	h.Write(namespace[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// deleteWeaviateClass issues DELETE /v1/schema/{class} for post-test cleanup.
// Errors are logged but do not fail the test.
func deleteWeaviateClass(t *testing.T, host, apiKey, className string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/v1/schema/%s", host, className), nil)
	if err != nil {
		t.Logf("cleanup: build delete request: %v", err)
		return
	}
	if apiKey != "" {
		req.Header.Set("X-Weaviate-Api-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("cleanup: delete class %s: %v", className, err)
		return
	}
	resp.Body.Close()
	t.Logf("cleanup: Weaviate class %s deleted (status %d)", className, resp.StatusCode)
}
