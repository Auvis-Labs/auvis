package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

var skipDestinationCheck bool

func init() {
	devCmd.Flags().BoolVar(&skipDestinationCheck, "skip-destination-check", false, "Skip startup reachability/auth validation for configured destinations")
}

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

	// --- Automatic Policy Validation & Syntax Check ---
	valErrors, valWarnings, _ := performPolicyValidation(cfg.PoliciesDir)
	for _, w := range valWarnings {
		slog.Warn("Policy validation warning", "file", w.File, "policy", w.Policy, "field", w.Field, "message", w.Message)
	}
	if len(valErrors) > 0 {
		for _, err := range valErrors {
			slog.Error("Policy validation error", "file", err.File, "policy", err.Policy, "field", err.Field, "message", err.Message)
		}
		return fmt.Errorf("FATAL: Policy validation failed on startup (%d errors found): %s", len(valErrors), valErrors[0].Message)
	}

	// --- Audit logger ---
	auditLog, err := audit.NewDurabilityLogger(cfg.AuditFile, 512, cfg.AuditMaxSizeMB, cfg.AuditMaxAgeDays, cfg.AuditFsyncInterval)
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
	}

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
	var connectedNames []string
	if len(cfg.Destinations) > 0 {
		dests, names, cbConfigs, err := buildDestinations(cfg.Destinations, skipDestinationCheck)
		if err != nil {
			slog.Error("FATAL: Destination startup validation failed — aborting gateway start", "error", err)
			return fmt.Errorf("FATAL: Destination startup validation failed — aborting gateway start: %w", err)
		}
		connectedNames = names

		if len(dests) == 1 {
			// Single destination — use BreakerDestination directly with configured parameters.
			dest = destination.NewBreakerDestination(dests[0], names[0], cbConfigs[0])
			slog.Info("Destination configured", "type", names[0], "max_failures", cbConfigs[0].MaxFailures, "cooldown", cbConfigs[0].Cooldown.String())
		} else if len(dests) > 1 {
			// Multiple destinations — wrap in a fan-out MultiDestination with per-destination circuit breakers.
			dest = destination.NewMultiDestination(dests, names, cbConfigs...)
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

	// Health check endpoint — exposes gateway status and per-destination circuit breaker states
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		cbStates := make(map[string]string)
		if exporter, ok := dest.(interface{ GetBreakerStates() map[string]string }); ok {
			cbStates = exporter.GetBreakerStates()
		}

		resp := map[string]any{
			"status":           "ok",
			"service":          "auvis",
			"circuit_breakers": cbStates,
		}

		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
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

	// Print clean startup summary on success
	printStartupSummary(cfg.Port, cfg.UpstreamURL, n, connectedNames, skipDestinationCheck, cfg.AuditFile)

	go func() {
		slog.Info("AUVIS gateway server running", "port", cfg.Port, "upstream", cfg.UpstreamURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server critical failure", "error", err)
			os.Exit(1)
		}
	}()

	shutdownSec := cfg.ShutdownTimeoutSec
	if shutdownSec <= 0 {
		shutdownSec = 30
	}

	<-stop
	slog.Info("Initiating graceful gateway shutdown", "timeout_seconds", shutdownSec)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSec)*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Warn("Graceful shutdown timeout exceeded — force-exiting server",
			"timeout_seconds", shutdownSec,
			"error", err,
			"total_requests_processed", atomic.LoadInt64(&requestCount),
		)
	}

	if dest != nil {
		if err := dest.Flush(); err != nil {
			slog.Error("Failed to flush destination buffer during shutdown", "error", err)
		}
	}

	auditLog.Close()
	slog.Info("Gateway gracefully terminated", "total_requests", atomic.LoadInt64(&requestCount))

	return nil
}

// buildDestinations constructs and connects all configured destination drivers.
// Returns parallel slices of connected destinations, their type labels, and circuit breaker configs.
// All successfully connected destinations are registered for defer Close via the caller.
func buildDestinations(dcs []config.DestinationConfig, skipCheck bool) ([]pipeline.Destination, []string, []destination.CBConfig, error) {
	dests := make([]pipeline.Destination, 0, len(dcs))
	names := make([]string, 0, len(dcs))
	cbConfigs := make([]destination.CBConfig, 0, len(dcs))

	for i, dc := range dcs {
		d, err := buildDestination(dc)
		if err != nil {
			for _, prev := range dests {
				_ = prev.Close()
			}
			return nil, nil, nil, fmt.Errorf("destination[%d] (%s) configuration invalid: %w", i, dc.Type, err)
		}
		if err := d.Connect(); err != nil {
			if skipCheck {
				slog.Warn("Destination startup connection check failed — proceeding anyway (--skip-destination-check enabled)",
					"destination_index", i,
					"destination_type", dc.Type,
					"error", err,
				)
			} else {
				for _, prev := range dests {
					_ = prev.Close()
				}
				slog.Error("Destination failed startup reachability/auth check",
					"destination_index", i,
					"destination_type", dc.Type,
					"error", err,
				)
				return nil, nil, nil, fmt.Errorf("destination[%d] (%s) failed startup connection check: %w", i, dc.Type, err)
			}
		}

		dests = append(dests, d)
		names = append(names, dc.Type)

		maxFailures := dc.CircuitBreaker.MaxFailures
		if maxFailures <= 0 {
			maxFailures = 5
		}
		cooldownSec := dc.CircuitBreaker.CooldownSeconds
		if cooldownSec <= 0 {
			cooldownSec = 30
		}
		cbConfigs = append(cbConfigs, destination.CBConfig{
			MaxFailures: maxFailures,
			Cooldown:    time.Duration(cooldownSec) * time.Second,
		})
	}

	return dests, names, cbConfigs, nil
}

func printStartupSummary(port, upstream string, policyCount int, destNames []string, skipDestCheck bool, auditFile string) {
	destStr := "None configured"
	if len(destNames) > 0 {
		destStr = fmt.Sprintf("%d active (%s)", len(destNames), strings.Join(destNames, ", "))
		if skipDestCheck {
			destStr += " [skip-destination-check]"
		}
	}

	fmt.Println()
	bold.Println(" AUVIS Gateway — Startup Summary")
	separator()
	fmt.Printf("  • Listening Port:     :%s\n", port)
	fmt.Printf("  • Upstream Target:    %s\n", upstream)
	fmt.Printf("  • Policies Loaded:    %d policy file(s) validated & active\n", policyCount)
	fmt.Printf("  • Destination Sinks:  %s\n", destStr)
	fmt.Printf("  • Audit Log File:     %s\n", auditFile)
	separator()
	fmt.Println()
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
