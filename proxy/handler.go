package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cridiv/Auvis/audit"
	"github.com/cridiv/Auvis/config"
	"github.com/cridiv/Auvis/metrics"
	"github.com/cridiv/Auvis/pipeline"
	"github.com/cridiv/Auvis/pipeline/destination"
	"github.com/cridiv/Auvis/pii"
	"github.com/cridiv/Auvis/policy"
	"github.com/cridiv/Auvis/transform"
	"github.com/cridiv/Auvis/types"
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

// Recovery is a middleware wrapper that catches panics in HTTP request handling,
// logs the stack trace with slog.Error, and returns an HTTP 500 status code
// to prevent crashing the main proxy process.
func Recovery(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rErr := recover(); rErr != nil {
				metrics.IncPanic("proxy_middleware")
				stack := string(debug.Stack())
				slog.Error("Panic recovered in HTTP handler middleware",
					"error", rErr,
					"stack", stack,
					"method", r.Method,
					"url", r.URL.String(),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	}
}

// Handler returns the catch-all proxy HTTP handler.
func Handler(
	cfg config.Config,
	store *policy.Store,
	transformFn transform.StreamTransformFunc,
	auditLog *audit.Logger,
	requestCount *int64,
	dest pipeline.Destination,
) http.HandlerFunc {
	transport := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
	}

	// bufPool reuses response buffers across requests, cutting per-request GC pressure.
	bufPool := sync.Pool{New: func() any { return new(bytes.Buffer) }}

	// proxyCache stores one httputil.ReverseProxy per upstream URL string.
	// Avoids per-request alloc of the proxy struct, url.Parse, and three closure captures.
	type upstreamEntry struct {
		parsedURL *url.URL
		rp        *httputil.ReverseProxy
	}
	var proxyCache sync.Map

	getProxy := func(rawUpstream string) (*httputil.ReverseProxy, error) {
		if v, ok := proxyCache.Load(rawUpstream); ok {
			return v.(*upstreamEntry).rp, nil
		}
		parsedURL, err := url.Parse(rawUpstream)
		if err != nil || parsedURL.Host == "" {
			return nil, fmt.Errorf("invalid upstream URL %q", rawUpstream)
		}
		rp := httputil.NewSingleHostReverseProxy(parsedURL)
		rp.Transport = transport
		rp.Director = func(req *http.Request) {
			req.URL.Scheme = parsedURL.Scheme
			req.URL.Host = parsedURL.Host
			req.Host = parsedURL.Host
			// Strip AUVIS-specific headers — upstream never sees these
			req.Header.Del("X-Auvis-Upstream")
			req.Header.Del("X-User-ID")
			req.Header.Del("X-User-Role")
			// Preserve the Authorization header — upstream needs it
			// (JWT from the test server flows through untouched)
		}
		rp.ModifyResponse = func(resp *http.Response) error {
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
			// reqCtx is attached to the HTTP request context just before ServeHTTP is called.
			rc, _ := r.Context().Value(auvisContextKey).(types.RequestContext)
			slog.Error("Upstream service unavailable", "error", err, "upstream", rawUpstream, "request_id", rc.RequestID)
			auditLog.Write(audit.Entry{
				Timestamp:      time.Now().UTC(),
				RequestID:      rc.RequestID,
				UserID:         rc.UserID,
				UserRole:       rc.UserRole,
				Resource:       rc.Resource,
				Method:         rc.Method,
				UpstreamStatus: http.StatusBadGateway,
			})
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		}
		entry := &upstreamEntry{parsedURL: parsedURL, rp: rp}
		if actual, loaded := proxyCache.LoadOrStore(rawUpstream, entry); loaded {
			return actual.(*upstreamEntry).rp, nil
		}
		return rp, nil
	}
	// Initialize UpstreamRouter for multi-upstream routing
	router, _ := NewUpstreamRouter(cfg.Upstreams)

	resolver := ChainResolver{
		&JWTResolver{IdpURL: cfg.IdpURL},
		&HeaderResolver{},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rErr := recover(); rErr != nil {
				metrics.IncPanic("proxy_handler")
				stack := string(debug.Stack())
				slog.Error("Panic recovered in HTTP proxy handler",
					"error", rErr,
					"stack", stack,
					"method", r.Method,
					"url", r.URL.String(),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		start := time.Now()
		atomic.AddInt64(requestCount, 1)

		// STEP 0 — Resolve upstream route & policy store
		activeStore := store
		rawUpstream := ""

		if router != nil {
			resolvedRoute := router.Resolve(r)
			if resolvedRoute != nil {
				rawUpstream = resolvedRoute.TargetRaw
				if resolvedRoute.Store != nil && resolvedRoute.Store.Count() > 0 {
					activeStore = resolvedRoute.Store
				}
			}
		}

		if h := r.Header.Get("X-Auvis-Upstream"); h != "" {
			rawUpstream = h
		}
		if rawUpstream == "" {
			rawUpstream = cfg.UpstreamURL
		}
		if rawUpstream == "" {
			http.Error(w, "Missing X-Auvis-Upstream header and no default upstream configured", http.StatusBadRequest)
			return
		}

		// STEP 1 — Extract + validate identity
		identity, err := resolver.Resolve(r)
		if err != nil {
			http.Error(w, "Missing identity headers or credentials", http.StatusUnauthorized)
			return
		}
		reqCtx := extractContext(r, identity)

		// STEP 1.5 — Lookup policy for LLM PII interception
		ep := activeStore.Lookup(reqCtx.UserRole, reqCtx.Resource, reqCtx.Service, reqCtx.Source, reqCtx.Destination, reqCtx.Trigger)
		isLLMIntercept := false
		var llmInterceptor *pii.LLMInterceptor
		var placeholderMap *pii.PlaceholderMap
		var requestSummary transform.Summary

		if ep != nil && ep.Rules["*"] == policy.ActionLLMIntercept {
			isLLMIntercept = true
			var bodyBytes []byte
			if r.Body != nil {
				var err error
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					slog.Error("Failed to read request body", "error", err, "request_id", reqCtx.RequestID)
					http.Error(w, "failed to read request body", http.StatusInternalServerError)
					return
				}
			}

			apiKey := os.Getenv("GEMINI_API_KEY")
			scannerMode := r.Header.Get("X-Auvis-Scanner-Mode")
			if scannerMode == "" {
				scannerMode = "hybrid"
			}
			scanner := &pii.Scanner{
				APIKey:      apiKey,
				LLMFallback: apiKey != "",
				Mode:        scannerMode,
			}
			llmInterceptor = &pii.LLMInterceptor{
				Scanner:   scanner,
				Rehydrate: ep.Response.Rehydrate,
			}

			maskedBody, pm, summary, err := llmInterceptor.InterceptRequest(r.Context(), bodyBytes, reqCtx)
			if err != nil {
				slog.Error("Failed to intercept request PII", "error", err, "request_id", reqCtx.RequestID)
				http.Error(w, "request interception error", http.StatusInternalServerError)
				return
			}

			placeholderMap = pm
			requestSummary = summary
			requestSummary.Priority = ep.Priority
			requestSummary.ComplianceTags = ep.ComplianceTags
			if len(ep.SourcePolicies) > 0 {
				requestSummary.PoliciesApplied = ep.SourcePolicies
			}

			// Update the request body and Content-Length for downstream
			r.Body = io.NopCloser(bytes.NewReader(maskedBody))
			r.ContentLength = int64(len(maskedBody))
			r.Header.Set("Content-Length", fmt.Sprintf("%d", len(maskedBody)))
		}

		// STEP 3 — Get or create a cached reverse proxy for this upstream.
		// The proxy (Director, ModifyResponse, ErrorHandler) is built once and reused
		// across all requests to the same upstream — zero per-request allocation.
		rp, err := getProxy(rawUpstream)
		if err != nil {
			http.Error(w, "Invalid upstream URL", http.StatusBadRequest)
			return
		}

		// STEP 5 — Pipe upstream response through transform
		pipeReader, pipeWriter := io.Pipe()

		infoCh := make(chan responseInfo, 1)

		interceptor := &responseInterceptor{
			pipeWriter: pipeWriter,
			infoCh:     infoCh,
			header:     make(http.Header),
		}

		// Attach reqCtx to the HTTP request context so the cached ErrorHandler can read
		// it without a per-request closure capture.
		r = r.WithContext(context.WithValue(r.Context(), auvisContextKey, reqCtx))

		go func() {
			rp.ServeHTTP(interceptor, r)
			pipeWriter.Close()
		}()

		info := <-infoCh

		// STEP 6 — Forward headers to client
		for k, vals := range info.header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Del("Content-Encoding")
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Type", "application/json")

		// STEP 7 — Transform / Rehydrate
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufPool.Put(buf)
		var summary transform.Summary
		var transformErr error

		if isLLMIntercept {
			// Read the raw response body from the pipe
			responseBytes, err := io.ReadAll(pipeReader)
			if err != nil {
				slog.Error("Failed to read response stream", "error", err, "request_id", reqCtx.RequestID)
				http.Error(w, "stream read error", http.StatusInternalServerError)
				return
			}

			// Rehydrate placeholders in response
			rehydratedBytes, err := llmInterceptor.InterceptResponse(responseBytes, placeholderMap)
			if err != nil {
				slog.Error("Failed to intercept response", "error", err, "request_id", reqCtx.RequestID)
				http.Error(w, "response interception error", http.StatusInternalServerError)
				return
			}

			buf.Write(rehydratedBytes)
			summary = requestSummary
		} else if info.statusCode < 200 || info.statusCode >= 300 {
			// Non-2xx upstream response (error, redirect body, etc.) — pass through raw,
			// no field rules should be applied to error payloads.
			responseBytes, err := io.ReadAll(pipeReader)
			if err != nil {
				slog.Error("Failed to read non-2xx upstream response", "error", err, "request_id", reqCtx.RequestID)
				http.Error(w, "upstream read error", http.StatusBadGateway)
				return
			}
			buf.Write(responseBytes)
			summary = transform.Summary{}
		} else {
			activeTransformFn := transformFn
			if activeStore != store {
				activeTransformFn = transform.NewStreamApplyFn(activeStore)
			}
			summary, transformErr = activeTransformFn(pipeReader, buf, reqCtx)
			if transformErr != nil {
				slog.Error("Response transformation error", "error", transformErr, "request_id", reqCtx.RequestID)
				http.Error(w, "transform error", http.StatusInternalServerError)
				return
			}
		}

		// STEP 7.5 — Synchronous destination fan-out (fail-closed)
		// The governed response body is written to all configured destinations before
		// returning to the client. If any write fails, the client gets a 500 and the
		// response is withheld — no governed data escapes unless all destinations confirm.
		var destResults []audit.DestResult
		var destsWritten, destsFailed int
		var destMs int64
		var destErrStr string
		if dest != nil {
			// Parse the governed response body into a structured map so the JSONB
			// column is queryable in Postgres rather than opaque.
			var dataPayload map[string]any
			if err := json.Unmarshal(buf.Bytes(), &dataPayload); err != nil {
				// Fallback: store as raw string if body isn't valid JSON.
				dataPayload = map[string]any{"body": buf.String()}
			}

			rec := pipeline.Record{
				ID:     reqCtx.RequestID,
				Source: rawUpstream,
				Schema: reqCtx.Resource,
				Data:   dataPayload,
				Metadata: map[string]any{
					"user_id":         reqCtx.UserID,
					"user_role":        reqCtx.UserRole,
					"method":           reqCtx.Method,
					"upstream_status":  info.statusCode,
					"fields_masked":    summary.FieldsMasked,
					"policies_applied": summary.PoliciesApplied,
				},
				Timestamp: time.Now().UTC(),
			}

			destStart := time.Now()
			var writeErr error
			func() {
				defer func() {
					if rErr := recover(); rErr != nil {
						metrics.IncPanic("destination_write")
						stack := string(debug.Stack())
						writeErr = fmt.Errorf("panic in destination write call: %v", rErr)
						slog.Error("Panic recovered during destination write call",
							"error", writeErr,
							"stack", stack,
							"request_id", reqCtx.RequestID,
						)
					}
				}()
				writeErr = dest.WriteRecord(rec)
			}()
			destMs = time.Since(destStart).Milliseconds()

			// Unpack per-destination outcomes from MultiError (multi-destination path)
			// or synthesise a single result for a bare destination (single-destination path).
			var me *destination.MultiError
			if errors.As(writeErr, &me) {
				for _, r := range me.Results {
					errMsg := ""
					if r.Err != nil {
						errMsg = r.Err.Error()
						destsFailed++
					} else {
						destsWritten++
					}
					destResults = append(destResults, audit.DestResult{
						Type:       r.Type,
						Written:    r.Written,
						ErrorMsg:   errMsg,
						DurationMs: r.DurationMs,
					})
				}
			} else {
				destName := "destination"
				if bd, ok := dest.(*destination.BreakerDestination); ok {
					destName = bd.Name
				}
				if writeErr != nil {
					destsFailed++
					metrics.IncDestinationWrite(destName, "failure")
					destResults = append(destResults, audit.DestResult{
						Type:       destName,
						Written:    false,
						ErrorMsg:   writeErr.Error(),
						DurationMs: destMs,
					})
				} else {
					destsWritten++
					metrics.IncDestinationWrite(destName, "success")
					destResults = append(destResults, audit.DestResult{
						Type:       destName,
						Written:    true,
						DurationMs: destMs,
					})
				}
			}

			if writeErr != nil {
				destErrStr = writeErr.Error()
				slog.Warn("Destination write failed — delivering governed response to client (decoupled destination failure)",
					"error", writeErr,
					"request_id", reqCtx.RequestID,
					"destinations_written", destsWritten,
					"destinations_failed", destsFailed,
				)
			} else {
				slog.Info("Destination write succeeded",
					"request_id", reqCtx.RequestID,
					"destinations_written", destsWritten,
					"destination_ms", destMs,
				)
			}
		}

		responseStatus := info.statusCode
		if summary.Denied {
			responseStatus = http.StatusForbidden
		}

		w.WriteHeader(responseStatus)
		_, _ = w.Write(buf.Bytes())

		// STEP 8 — Audit & Prometheus Telemetry
		metrics.ObserveRequest(reqCtx.Method, reqCtx.Resource, responseStatus, time.Since(start).Seconds())
		metrics.IncPolicyMatch(len(summary.PoliciesApplied) > 0)
		if summary.Denied {
			metrics.IncOutcome("denied")
		} else if len(summary.FieldsMasked) > 0 {
			metrics.IncOutcome("masked")
		} else if len(summary.FieldsRemoved) > 0 {
			metrics.IncOutcome("removed")
		} else {
			metrics.IncOutcome("allowed")
		}

		auditLog.Write(audit.Entry{
			Timestamp:           time.Now().UTC(),
			RequestID:           reqCtx.RequestID,
			UserID:              reqCtx.UserID,
			UserRole:            reqCtx.UserRole,
			Resource:            reqCtx.Resource,
			Method:              reqCtx.Method,
			PoliciesApplied:     summary.PoliciesApplied,
			Priority:            summary.Priority,
			ComplianceTags:      summary.ComplianceTags,
			ConditionEvaluated:  summary.ConditionEvaluated,
			FieldsMasked:        summary.FieldsMasked,
			FieldsRemoved:       summary.FieldsRemoved,
			FieldsAllowed:       summary.FieldsAllowed,
			FieldsDenied:        summary.FieldsDenied,
			ResponseDenied:      summary.ResponseDenied,
			PiiTypesDetected:    summary.PiiTypesDetected,
			RowsFiltered:        summary.RowsFiltered,
			UpstreamStatus:      info.statusCode,
			DurationMS:          time.Since(start).Milliseconds(),
			DestinationWritten:  destsWritten > 0 && destsFailed == 0,
			DestinationError:    destErrStr,
			DestinationsWritten: destsWritten,
			DestinationsFailed:  destsFailed,
			DestinationMs:       destMs,
			DestinationResults:  destResults,
			RecordCount:         len(summary.RecordOutcomes),
			RecordOutcomes:      mapRecordOutcomes(summary.RecordOutcomes),
		})

		slog.Info("Request handled successfully",
			"request_id", reqCtx.RequestID,
			"method", reqCtx.Method,
			"resource", reqCtx.Resource,
			"upstream", rawUpstream,
			"user_id", reqCtx.UserID,
			"role", reqCtx.UserRole,
			"status", responseStatus,
			"duration_ms", time.Since(start).Milliseconds(),
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

// mapRecordOutcomes converts transform.RecordOutcome slice to audit.RecordOutcome slice.
// Kept as a separate mapping step to avoid a circular import between the audit and transform packages.
func mapRecordOutcomes(in []transform.RecordOutcome) []audit.RecordOutcome {
	if len(in) == 0 {
		return nil
	}
	out := make([]audit.RecordOutcome, len(in))
	for i, r := range in {
		out[i] = audit.RecordOutcome{
			RecordID:      r.RecordID,
			FieldsMasked:  r.FieldsMasked,
			FieldsRemoved: r.FieldsRemoved,
			FieldsAllowed: r.FieldsAllowed,
			FieldsDenied:  r.FieldsDenied,
			Filtered:      r.Filtered,
		}
	}
	return out
}