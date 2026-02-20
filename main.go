package main

import (
	"log"
	"net/http"

	"hebris/config"
	"hebris/proxy"
)

func main() {
	cfg := config.LoadConfig()

	handler := proxy.Handler(cfg)

	http.HandleFunc("/", handler)

	log.Printf("AUVIS running on port %s → proxying to %s", cfg.Port, cfg.UpstreamURL)

	err := http.ListenAndServe(":"+cfg.Port, nil)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}