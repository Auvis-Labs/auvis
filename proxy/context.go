package proxy

import (
	"fmt"
	"net/http"
	"time"

	"hebris/types"
)

func extractContext(r *http.Request) types.RequestContext {
	return types.RequestContext{
		RequestID: fmt.Sprintf("req-%d", time.Now().UnixNano()),
		UserID:    r.Header.Get("X-User-ID"),
		UserRole:  r.Header.Get("X-User-Role"),
		Resource:  r.URL.Path,
		Method:    r.Method,
	}
}