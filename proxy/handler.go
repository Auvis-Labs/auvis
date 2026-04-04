package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"hebris/audit"
	"hebris/config"
	"hebris/transform"
	"hebris/types"
)

type contextKey string

const auvisContextKey contextKey = "auvis_ctx"

// bodyCapture intercepts the upstream response body before it reaches the client.
type streamCapture struct {
	header     http.Header
	statusCode int
	bodyReader io.ReadCloser
}

func newStreamCapture() *streamCapture {
	return &streamCapture{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

// GetRequestContext retrieves the request context attached by the handler.
func GetRequestContext(r *http.Request) (types.RequestContext, bool) {
	ctx, ok := r.Context().Value(auvisContextKey).(types.RequestContext)
	return ctx, ok
}

// Handler wires the proxy, transform, and audit logger together.
// requestCount is incremented atomically on every request.
func Handler(
	cfg config.Config,
	transformFn transform.StreamTransformFunc,
	auditLog *audit.Logger,
	requestCount *int64,
) http.HandlerFunc {
	upstreamURL, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		log.Fatalf("Invalid upstream URL: %v", err)
	}

	rp := httputil.NewSingleHostReverseProxy(upstreamURL)
	originalDirector := rp.Director

	rp.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = upstreamURL.Host
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Del("X-User-ID")
		req.Header.Del("X-User-Role")
	}

	rp.ModifyResponse = func(resp *http.Response) error {
		// Decompress if needed — give transform a clean reader
		if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
			gr, err := gzip.NewReader(resp.Body)
			if err != nil {
				return fmt.Errorf("gzip reader: %w", err)
			}
			resp.Body = gr
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
		}
		return nil
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[AUVIS] upstream error: %v", err)
		if reqCtx, ok := GetRequestContext(r); ok {
			auditLog.Write(audit.Entry{
				Timestamp:      time.Now().UTC(),
				RequestID:      reqCtx.RequestID,
				UserID:         reqCtx.UserID,
				UserRole:       reqCtx.UserRole,
				Resource:       reqCtx.Resource,
				Method:         reqCtx.Method,
				UpstreamStatus: http.StatusBadGateway,
			})
		}
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		atomic.AddInt64(requestCount, 1)

		// STEP 1 — Extract + validate identity
		reqCtx := extractContext(r)
		if reqCtx.UserID == "" || reqCtx.UserRole == "" {
			http.Error(w, "Missing identity headers", http.StatusUnauthorized)
			return
		}

		// STEP 2 — Attach context
		ctx := context.WithValue(r.Context(), auvisContextKey, reqCtx)
		r = r.WithContext(ctx)

		// STEP 3 — Capture upstream response headers + status,
		//          but intercept the body via ModifyResponse (set above).
		//          We use a pipe: upstream writes to pipeWriter,
		//          transform reads from pipeReader.
		pipeReader, pipeWriter := io.Pipe()

		// responseInfo carries status/headers back from ModifyResponse
		// to the main goroutine.
		type responseInfo struct {
			statusCode int
			header     http.Header
		}
		infoCh := make(chan responseInfo, 1)

		// Wrap the real ResponseWriter so the reverse proxy can write
		// headers/status, while we intercept the body via the pipe.
		interceptor := &responseInterceptor{
			pipeWriter: pipeWriter,
			infoCh:     infoCh,
		}

		// STEP 4 — Run the upstream request in a goroutine.
		//          ServeHTTP blocks until the full response is sent,
		//          but because we pipe the body, it streams naturally.
		go func() {
			rp.ServeHTTP(interceptor, r)
			pipeWriter.Close() // signals EOF to the transform
		}()

		// STEP 5 — Wait for headers from the interceptor
		info := <-infoCh

		// STEP 6 — Forward headers to client (strip encoding, length)
		for k, vals := range info.header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Del("Content-Encoding")
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Type", "application/json")

		// STEP 7 — Determine status before writing header
		responseStatus := info.statusCode

		// STEP 8 — Run transform: reads from pipe, writes to client
		// We need to capture the summary, so buffer the output only
		// if a deny check is needed. For now: transform into a buf,
		// check deny, then write. True zero-copy comes when we have
		// per-record streaming — this is the correct interface already.
		var buf bytes.Buffer
		summary, err := transformFn(pipeReader, &buf, reqCtx)
		if err != nil {
			log.Printf("[AUVIS] transform error: %v", err)
			http.Error(w, "transform error", http.StatusInternalServerError)
			return
		}

		if summary.Denied {
			responseStatus = http.StatusForbidden
		}

		w.WriteHeader(responseStatus)
		_, _ = w.Write(buf.Bytes())

		// STEP 9 — Audit (non-blocking)
		auditLog.Write(audit.Entry{
			Timestamp:       time.Now().UTC(),
			RequestID:       reqCtx.RequestID,
			UserID:          reqCtx.UserID,
			UserRole:        reqCtx.UserRole,
			Resource:        reqCtx.Resource,
			Method:          reqCtx.Method,
			PoliciesApplied: summary.PoliciesApplied,
			FieldsMasked:    summary.FieldsMasked,
			UpstreamStatus:  info.statusCode,
			DurationMS:      time.Since(start).Milliseconds(),
		})

		log.Printf(
			"[AUVIS] request_id=%s method=%s resource=%s user=%s role=%s status=%d duration=%v",
			reqCtx.RequestID, reqCtx.Method, reqCtx.Resource,
			reqCtx.UserID, reqCtx.UserRole, responseStatus, time.Since(start),
		)
	}
}

// responseInterceptor implements http.ResponseWriter.
// It forwards headers/status via infoCh and pipes the body.
type responseInterceptor struct {
	pipeWriter  *io.PipeWriter
	infoCh      chan responseInfo
	header      http.Header
	wroteHeader bool
}

type responseInfo struct {
	statusCode int
	header     http.Header
}

func newResponseInterceptor(pw *io.PipeWriter, ch chan responseInfo) *responseInterceptor {
	return &responseInterceptor{
		pipeWriter: pw,
		infoCh:     ch,
		header:     make(http.Header),
	}
}

func (ri *responseInterceptor) Header() http.Header { return ri.header }

func (ri *responseInterceptor) WriteHeader(code int) {
	if ri.wroteHeader {
		return
	}
	ri.wroteHeader = true
	ri.infoCh <- responseInfo{statusCode: code, header: ri.header.Clone()}
}

func (ri *responseInterceptor) Write(p []byte) (int, error) {
	if !ri.wroteHeader {
		ri.WriteHeader(http.StatusOK)
	}
	return ri.pipeWriter.Write(p)
}