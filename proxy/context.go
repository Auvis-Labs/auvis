package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cridiv/Auvis/types"
)

func extractContext(r *http.Request, idpURL string) types.RequestContext {
	// Try parsing JWT first
	jwtClaims := parseJWTClaims(r.Header.Get("Authorization"))

	// Determine identity from resolution chain:
	userID := ""
	userRole := ""

	// 1. JWT Claims first
	if val, ok := jwtClaims["sub"]; ok && val != "" {
		userID = val
	} else if val, ok := jwtClaims["uid"]; ok && val != "" {
		userID = val
	} else if val, ok := jwtClaims["userid"]; ok && val != "" {
		userID = val
	}

	if val, ok := jwtClaims["role"]; ok && val != "" {
		userRole = val
	} else if val, ok := jwtClaims["userrole"]; ok && val != "" {
		userRole = val
	}

	// 2. Fallback to explicit X-User-* headers
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	if userRole == "" {
		userRole = r.Header.Get("X-User-Role")
	}

	attrs := make(map[string]string)

	// 1. Add explicit X-User-* headers (excluding ID and Role which are identity)
	for h, values := range r.Header {
		if strings.HasPrefix(strings.ToLower(h), "x-user-") && len(values) > 0 {
			lowerH := strings.ToLower(h)
			if lowerH != "x-user-id" && lowerH != "x-user-role" {
				fieldName := lowerH[7:]
				attrs[fieldName] = values[0]
			}
		}
	}

	// 2. Overlay JWT claims
	for k, v := range jwtClaims {
		attrs[k] = v
	}

	// 3. Overlay optional Identity Provider lookup
	if idpURL != "" && userID != "" {
		idpAttrs := resolveIdpAttributes(userID, idpURL)
		for k, v := range idpAttrs {
			attrs[k] = v
		}
	}

	// Ensure role and id are always populated to avoid "missing attribute" errors on standard identity fields
	attrs["role"] = userRole
	attrs["id"] = userID

	// Extract service name
	service := r.Header.Get("X-Auvis-Service")
	if service == "" {
		service = r.Header.Get("X-Service-Name")
	}
	if service == "" {
		service = r.Header.Get("X-Service")
	}
	if service == "" {
		if upstream := r.Header.Get("X-Auvis-Upstream"); upstream != "" {
			if parsed, err := url.Parse(upstream); err == nil && parsed.Host != "" {
				host := parsed.Host
				if idx := strings.Index(host, ":"); idx != -1 {
					host = host[:idx]
				}
				service = host
			}
		}
	}

	// Extract fields
	var fields []string
	if fieldsStr := r.URL.Query().Get("fields"); fieldsStr != "" {
		for _, f := range strings.Split(fieldsStr, ",") {
			trimmed := strings.TrimSpace(f)
			if trimmed != "" {
				fields = append(fields, trimmed)
			}
		}
	} else if fieldsHeader := r.Header.Get("X-Auvis-Fields"); fieldsHeader != "" {
		for _, f := range strings.Split(fieldsHeader, ",") {
			trimmed := strings.TrimSpace(f)
			if trimmed != "" {
				fields = append(fields, trimmed)
			}
		}
	}

	return types.RequestContext{
		RequestID:      fmt.Sprintf("req-%d", time.Now().UnixNano()),
		UserID:         userID,
		UserRole:       userRole,
		Resource:       r.URL.Path,
		Method:         r.Method,
		Service:        service,
		Fields:         fields,
		UserAttributes: attrs,
		Trigger:        "read",
	}
}

// parseJWTClaims decodes standard unverified JWT payload claims from an Authorization Bearer token.
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

	// Decode payload segment
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

// resolveIdpAttributes queries the optional IdP server synchronously to lookup dynamic user attributes.
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