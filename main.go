package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"hebris/audit"
	"hebris/config"
	"hebris/policy"
	"hebris/proxy"
	"hebris/transform"
)

func main() {
	startTime := time.Now()
	cfg := config.LoadConfig()

	// --- Audit logger ---
	auditLog, err := audit.NewLogger("./audit.jsonl", 512)
	if err != nil {
		log.Fatalf("[MAIN] failed to open audit log: %v", err)
	}
	log.Printf("[MAIN] audit log → ./audit.jsonl")

	// --- Policy store ---
	store := policy.NewStore()
	n, err := store.LoadDir("./policies")
	if err != nil {
		log.Fatalf("[MAIN] failed to load policies: %v", err)
	}
	log.Printf("[MAIN] %d effective policies loaded", n)

	if err := store.WatchDir("./policies"); err != nil {
		log.Fatalf("[MAIN] failed to watch policy dir: %v", err)
	}

	// --- Request counter — incremented by the proxy handler ---
	var requestCount int64

	// --- Wire everything together ---
	transformFn := transform.NewStreamApplyFn(store)

	mux := http.NewServeMux()

	// Internal IRIS endpoints — registered before the catch-all proxy
	audit.RegisterHandlers(mux, "./audit.jsonl", startTime, &requestCount)

	// Catch-all proxy handler
	mux.Handle("/", proxy.Handler(cfg, transformFn, auditLog, &requestCount))

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// --- Graceful shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("AUVIS running on port %s → proxying to %s", cfg.Port, cfg.UpstreamURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	log.Printf("[MAIN] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[MAIN] shutdown error: %v", err)
	}

	auditLog.Close()
	log.Printf("[MAIN] goodbye. total requests served: %d", atomic.LoadInt64(&requestCount))
}