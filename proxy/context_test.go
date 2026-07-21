package proxy

import (
	"encoding/base64"
	"net/http"
	"testing"
)

func TestExtractContextResolutionChain(t *testing.T) {
	// 1. Test standard headers fallback
	req, _ := http.NewRequest("GET", "/users", nil)
	req.Header.Set("X-User-ID", "header-123")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("X-User-Region", "EU")

	ctx := extractContext(req, "")
	if ctx.UserID != "header-123" {
		t.Errorf("expected UserID='header-123', got %q", ctx.UserID)
	}
	if ctx.UserRole != "admin" {
		t.Errorf("expected UserRole='admin', got %q", ctx.UserRole)
	}
	if ctx.UserAttributes["region"] != "EU" {
		t.Errorf("expected region='EU', got %q", ctx.UserAttributes["region"])
	}

	// 2. Test JWT resolution (should override standard headers)
	// Base64URL-encoded dummy JWT payload: {"sub": "jwt-sub", "role": "jwt-role", "region": "US"}
	payload := `{"sub": "jwt-sub", "role": "jwt-role", "region": "US"}`
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	token := "header." + encodedPayload + ".signature"

	reqJWT, _ := http.NewRequest("GET", "/users", nil)
	reqJWT.Header.Set("Authorization", "Bearer "+token)
	// Add explicit headers to make sure JWT takes precedence
	reqJWT.Header.Set("X-User-ID", "header-123")
	reqJWT.Header.Set("X-User-Role", "admin")

	ctxJWT := extractContext(reqJWT, "")
	if ctxJWT.UserID != "jwt-sub" {
		t.Errorf("expected UserID='jwt-sub' from JWT, got %q", ctxJWT.UserID)
	}
	if ctxJWT.UserRole != "jwt-role" {
		t.Errorf("expected UserRole='jwt-role' from JWT, got %q", ctxJWT.UserRole)
	}
	if ctxJWT.UserAttributes["region"] != "US" {
		t.Errorf("expected region='US' from JWT, got %q", ctxJWT.UserAttributes["region"])
	}
}
