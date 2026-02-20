package proxy

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"hebris/config"
	"hebris/types"
)

type contextKey string

const auvisContextKey contextKey = "auvis_ctx"

// responseRecorder captures status code safely
type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.statusCode = code
		r.wroteHeader = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// Helper to retrieve context safely
func GetRequestContext(r *http.Request) (types.RequestContext, bool) {
	ctx, ok := r.Context().Value(auvisContextKey).(types.RequestContext)
	return ctx, ok
}

func Handler(cfg config.Config) http.HandlerFunc {
	upstreamURL, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		log.Fatalf("Invalid upstream URL: %v", err)
	}

	rp := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Preserve default behavior
	originalDirector := rp.Director

rp.Director = func(req *http.Request) {
    originalDirector(req)

    // Fix Host — this is almost certainly your 403 cause
    req.Host = upstreamURL.Host  // "jsonplaceholder.typicode.com"

    if req.Header.Get("User-Agent") == "" {
        req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
    }

    req.Header.Set("Accept", "application/json, text/plain, */*")
    req.Header.Set("Accept-Language", "en-US,en;q=0.9")
if req.Header.Get("Accept-Encoding") == "" {
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
}
    req.Header.Set("Connection", "keep-alive")
    req.Header.Set("Sec-Fetch-Dest", "empty")
    req.Header.Set("Sec-Fetch-Mode", "cors")
    req.Header.Set("Sec-Fetch-Site", "same-origin")

    req.Header.Del("X-User-ID")
    req.Header.Del("X-User-Role")
}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[AUVIS] upstream error: %v", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// STEP 1 — Extract Request Context
		reqCtx := extractContext(r)

		// STEP 2 — Validate identity (fail fast)
		if reqCtx.UserID == "" || reqCtx.UserRole == "" {
			http.Error(w, "Missing identity headers", http.StatusUnauthorized)
			return
		}

		// STEP 3 — Attach context to request
		ctx := context.WithValue(r.Context(), auvisContextKey, reqCtx)
		r = r.WithContext(ctx)

		// STEP 4 — Wrap response writer
		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// STEP 5 — Forward request to upstream
		rp.ServeHTTP(rec, r)

		// STEP 6 — Log request (structured)
		log.Printf(
			"[IRIS] request_id=%s method=%s resource=%s user=%s role=%s status=%d duration=%v",
			reqCtx.RequestID,
			reqCtx.Method,
			reqCtx.Resource,
			reqCtx.UserID,
			reqCtx.UserRole,
			rec.statusCode,
			time.Since(start),
		)
	}
}