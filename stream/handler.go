package stream

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Handler returns an http.HandlerFunc that flushes real-time transformation events over SSE.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ch := GlobalBroadcaster.Subscribe()
		defer GlobalBroadcaster.Unsubscribe(ch)

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				b, err := json.Marshal(event)
				if err == nil {
					fmt.Fprintf(w, "data: %s\n\n", string(b))
					flusher.Flush()
				}
			case <-r.Context().Done():
				return
			}
		}
	}
}
