package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// RegisterHandlers mounts the internal IRIS endpoints onto mux.
//
//	GET /iris/audit?limit=50       — last N audit entries
//	GET /iris/audit?role=analyst   — filter by role
//	GET /iris/audit?user=user-123  — filter by user
//	GET /iris/audit?resource=/users
//	GET /iris/status               — uptime + request count
func RegisterHandlers(mux *http.ServeMux, auditPath string, startTime time.Time, requestCounter *int64) {
	mux.HandleFunc("/iris/audit", auditHandler(auditPath))
	mux.HandleFunc("/iris/status", statusHandler(startTime, requestCounter))
}

func auditHandler(auditPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		filterRole := r.URL.Query().Get("role")
		filterUser := r.URL.Query().Get("user")
		filterResource := r.URL.Query().Get("resource")

		data, err := os.ReadFile(auditPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"entries": []Entry{},
					"total":   0,
				})
				return
			}
			http.Error(w, "could not read audit log", http.StatusInternalServerError)
			return
		}

		entries := parseEntries(data)

		filtered := make([]Entry, 0, len(entries))
		for _, e := range entries {
			if filterRole != "" && e.UserRole != filterRole {
				continue
			}
			if filterUser != "" && e.UserID != filterUser {
				continue
			}
			if filterResource != "" && e.Resource != filterResource {
				continue
			}
			filtered = append(filtered, e)
		}

		start := 0
		if len(filtered) > limit {
			start = len(filtered) - limit
		}
		page := filtered[start:]

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"entries": page,
			"total":   len(filtered),
			"limit":   limit,
			"filters": map[string]string{
				"role":     filterRole,
				"user":     filterUser,
				"resource": filterResource,
			},
		})
	}
}

func statusHandler(startTime time.Time, requestCounter *int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":         "ok",
			"uptime":         time.Since(startTime).Round(time.Second).String(),
			"started_at":     startTime.UTC().Format(time.RFC3339),
			"requests_total": *requestCounter,
		})
	}
}

// parseEntries splits the audit file into individual JSON blocks and parses each one.
// Blocks are separated by blank lines (\n\n).
func parseEntries(data []byte) []Entry {
	var entries []Entry

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var block bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if block.Len() > 0 {
				var e Entry
				if err := json.Unmarshal(block.Bytes(), &e); err == nil {
					entries = append(entries, e)
				}
				block.Reset()
			}
			continue
		}
		block.WriteString(line)
		block.WriteByte('\n')
	}

	// Handle last block if file doesn't end with blank line
	if block.Len() > 0 {
		var e Entry
		if err := json.Unmarshal(block.Bytes(), &e); err == nil {
			entries = append(entries, e)
		}
	}

	return entries
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("json marshal error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
}