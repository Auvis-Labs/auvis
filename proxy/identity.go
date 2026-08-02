package proxy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Identity represents the resolved caller context.
type Identity struct {
	UserID         string
	UserRole       string
	UserAttributes map[string]string
}

// IdentityResolver defines the pluggable interface for extracting caller credentials.
type IdentityResolver interface {
	Resolve(r *http.Request) (Identity, error)
}

// ErrIdentityNotResolved is returned when no resolver in the chain successfully extracts an identity.
var ErrIdentityNotResolved = errors.New("identity credentials not found or invalid")

// ChainResolver wraps a slice of IdentityResolvers and attempts each in sequence.
type ChainResolver []IdentityResolver

// Resolve runs each resolver in the chain. The first resolver that successfully resolves a non-empty identity wins.
func (c ChainResolver) Resolve(r *http.Request) (Identity, error) {
	for _, resolver := range c {
		identity, err := resolver.Resolve(r)
		if err == nil && identity.UserID != "" {
			return identity, nil
		}
	}
	return Identity{}, ErrIdentityNotResolved
}

// JWTResolver parses standard OIDC/OAuth JWT claims from the Authorization Bearer token header.
type JWTResolver struct {
	IdpURL string
}

// Resolve extracts the identity from standard JWT claims (sub, uid, roles, groups).
func (j *JWTResolver) Resolve(r *http.Request) (Identity, error) {
	claims := parseJWTClaims(r.Header.Get("Authorization"))
	if len(claims) == 0 {
		return Identity{}, nil
	}

	userID := getFirstClaim(claims, "sub", "uid", "userid")
	userRole := getFirstClaim(claims, "role", "userrole", "group")

	attrs := make(map[string]string)
	for k, v := range claims {
		if k != "sub" && k != "uid" && k != "userid" && k != "role" && k != "userrole" && k != "group" {
			attrs[k] = v
		}
	}

	if j.IdpURL != "" && userID != "" {
		idpAttrs := resolveIdpAttributes(userID, j.IdpURL)
		for k, v := range idpAttrs {
			attrs[k] = v
		}
	}

	return Identity{
		UserID:         userID,
		UserRole:       userRole,
		UserAttributes: attrs,
	}, nil
}

// HeaderResolver extracts identity from explicit custom gateway/proxy headers.
type HeaderResolver struct{}

// Resolve extracts the identity from X-User-ID and X-User-Role headers.
func (h *HeaderResolver) Resolve(r *http.Request) (Identity, error) {
	userID := r.Header.Get("X-User-ID")
	userRole := r.Header.Get("X-User-Role")

	if userID == "" {
		return Identity{}, nil
	}

	attrs := make(map[string]string)
	for k, values := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-user-") && len(values) > 0 {
			lowerK := strings.ToLower(k)
			if lowerK != "x-user-id" && lowerK != "x-user-role" {
				attrs[lowerK[7:]] = values[0]
			}
		}
	}

	return Identity{
		UserID:         userID,
		UserRole:       userRole,
		UserAttributes: attrs,
	}, nil
}

// --- Helper Functions ---

func getFirstClaim(claims map[string]string, keys ...string) string {
	for _, k := range keys {
		if val, ok := claims[k]; ok && val != "" {
			return val
		}
	}
	return ""
}

func parseJWTClaims(authHeader string) map[string]string {
	claims := make(map[string]string)
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return claims
	}
	token := authHeader[7:]
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims
	}

	payloadSegment := parts[1]
	if l := len(payloadSegment) % 4; l > 0 {
		payloadSegment += strings.Repeat("=", 4-l)
	}

	data, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		data, err = base64.StdEncoding.DecodeString(payloadSegment)
		if err != nil {
			return claims
		}
	}

	var rawClaims map[string]interface{}
	if err := json.Unmarshal(data, &rawClaims); err != nil {
		return claims
	}

	for k, v := range rawClaims {
		claims[strings.ToLower(k)] = fmt.Sprintf("%v", v)
	}
	return claims
}

func resolveIdpAttributes(userID string, idpURL string) map[string]string {
	attrs := make(map[string]string)
	if idpURL == "" || userID == "" {
		return attrs
	}

	client := &http.Client{Timeout: 2 * time.Second}
	reqURL := fmt.Sprintf("%s?user_id=%s", idpURL, userID)
	resp, err := client.Get(reqURL)
	if err != nil {
		log.Printf("[POLICY] IdP lookup failed: %v", err)
		return attrs
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var raw map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil {
			for k, v := range raw {
				attrs[strings.ToLower(k)] = fmt.Sprintf("%v", v)
			}
		}
	}
	return attrs
}
