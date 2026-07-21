package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Auvis-Labs/auvis/audit"
	"github.com/Auvis-Labs/auvis/config"
	"github.com/Auvis-Labs/auvis/pipeline"
	"github.com/Auvis-Labs/auvis/pipeline/destination"
	"github.com/Auvis-Labs/auvis/policy"
	"github.com/Auvis-Labs/auvis/proxy"
	"github.com/Auvis-Labs/auvis/transform"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the gateway with live policy reloading",
	RunE: func(cmd *cobra.Command, args []string) error {
		printInfo("Starting AUVIS gateway in development mode...")
		return runDev()
	},
}

func runDev() error {
	// Initialize default slog logger outputting structured text to stderr
	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(logHandler))

	startTime := time.Now()
	// Use global cfgFile flag
	cfg := config.LoadConfig(cfgFile)

	if cfg.UpstreamURL == "" {
		return fmt.Errorf("AUVIS_UPSTREAM environment variable is required")
	}

	// --- Audit logger ---
	auditLog, err := audit.NewRotatingLogger(cfg.AuditFile, 512, cfg.AuditMaxSizeMB, cfg.AuditMaxAgeDays)
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
	}
	slog.Info("Audit logger initialized", "destination", cfg.AuditFile)

	// --- Policy store ---
	store := policy.NewStore()
	n, err := store.LoadDir(cfg.PoliciesDir)
	if err != nil {
		return fmt.Errorf("failed to load policies: %w", err)
	}
	slog.Info("Policies loaded successfully", "count", n, "directory", cfg.PoliciesDir)

	if err := store.WatchDir(cfg.PoliciesDir); err != nil {
		return fmt.Errorf("failed to watch policy dir: %w", err)
	}

	// --- Request counter — incremented by the proxy handler ---
	var requestCount int64

	// --- Destinations (optional) — synchronously write governed records ---
	// Supports any number of destinations configured under `destinations:` in auvis.yaml.
	// Falls back gracefully to nil (no destination) if none are configured.
	var dest pipeline.Destination
	if len(cfg.Destinations) > 0 {
		dests, names, err := buildDestinations(cfg.Destinations)
		if err != nil {
			return err
		}

		if len(dests) == 1 {
			// Single destination — use directly, no MultiDestination wrapper overhead.
			dest = dests[0]
			slog.Info("Destination configured", "type", names[0])
		} else {
			// Multiple destinations — wrap in a fan-out MultiDestination.
			dest = destination.NewMultiDestination(dests, names)
			slog.Info("Destinations configured", "count", len(dests), "types", names)
		}
	}

	if dest != nil {
		defer func() {
			if err := dest.Close(); err != nil {
				slog.Error("Failed to close destination on exit", "error", err)
			}
		}()
	}

	// --- Wire everything together ---
	transformFn := transform.NewStreamApplyFn(store)

	mux := http.NewServeMux()

	// Internal AUVIS endpoints — registered before the catch-all proxy
	audit.RegisterHandlers(mux, cfg.AuditFile, startTime, &requestCount, store)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"auvis"}`))
	})

	// Catch-all proxy handler
	mux.Handle("/", proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, dest))

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// --- Graceful shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("AUVIS gateway server running", "port", cfg.Port, "upstream", cfg.UpstreamURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server critical failure", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Initiating graceful gateway shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown process encountered an error", "error", err)
	}

	auditLog.Close()
	slog.Info("Gateway gracefully terminated", "total_requests", atomic.LoadInt64(&requestCount))

	return nil
}

// buildDestinations constructs and connects all configured destination drivers.
// Returns parallel slices of connected destinations and their type labels.
// All successfully connected destinations are registered for defer Close via the caller.
func buildDestinations(dcs []config.DestinationConfig) ([]pipeline.Destination, []string, error) {
	dests := make([]pipeline.Destination, 0, len(dcs))
	names := make([]string, 0, len(dcs))

	for i, dc := range dcs {
		d, err := buildDestination(dc)
		if err != nil {
			return nil, nil, fmt.Errorf("destination[%d] %s: %w", i, dc.Type, err)
		}
		if err := d.Connect(); err != nil {
			return nil, nil, fmt.Errorf("destination[%d] %s connect: %w", i, dc.Type, err)
		}

		dests = append(dests, d)
		names = append(names, dc.Type)
	}

	return dests, names, nil
}

// buildDestination constructs (but does not connect) a single destination driver from config.
func buildDestination(dc config.DestinationConfig) (pipeline.Destination, error) {
	switch dc.Type {
	case "postgres":
		return destination.NewPostgresDestination(dc.Settings, dc.Auth, dc.TLS), nil
	case "pinecone":
		return destination.NewPineconeDestination(dc.Settings, dc.Auth, dc.TLS), nil
	case "weaviate":
		return destination.NewWeaviateDestination(dc.Settings, dc.Auth, dc.TLS), nil
	case "pgvector":
		return destination.NewPgVectorDestination(dc.Settings, dc.Auth, dc.TLS), nil
	default:
		return nil, fmt.Errorf("unknown destination type %q (supported: postgres, pinecone, weaviate, pgvector)", dc.Type)
	}
}
