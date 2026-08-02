package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cridiv/Auvis/types"
)

// extractContext builds the request context using request characteristics and the resolved caller identity.
func extractContext(r *http.Request, identity Identity) types.RequestContext {
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

	// Ensure role and id are always populated in user attributes map
	attrs := make(map[string]string)
	for k, v := range identity.UserAttributes {
		attrs[k] = v
	}
	attrs["role"] = identity.UserRole
	attrs["id"] = identity.UserID

	return types.RequestContext{
		RequestID:      fmt.Sprintf("req-%d", time.Now().UnixNano()),
		UserID:         identity.UserID,
		UserRole:       identity.UserRole,
		Resource:       r.URL.Path,
		Method:         r.Method,
		Service:        service,
		Fields:         fields,
		UserAttributes: attrs,
		Trigger:        "read",
	}
}