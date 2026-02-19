package main

import (
	"log"
	"net/http"
	"fmt"

	"iris/config"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("IRIS starting on :%s → upstream %s", cfg.Port, cfg.UpstreamURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Proxy target would be: %s%s", cfg.UpstreamURL, r.URL.Path)
	})

	addr := ":" + cfg.Port
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}