package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	pipelinecfg "github.com/cridiv/Auvis/pipeline/config"
)

// CircuitBreakerConfig specifies failure threshold and cooldown period for destination write protection.
type CircuitBreakerConfig struct {
	MaxFailures     int `yaml:"max_failures"`
	CooldownSeconds int `yaml:"cooldown_seconds"`
}

// UpstreamConfig defines a static upstream target service and its optional path/header routing prefix.
type UpstreamConfig struct {
	Name        string `yaml:"name" json:"name"`
	Prefix      string `yaml:"prefix" json:"prefix"`
	Header      string `yaml:"header" json:"header"`
	Target      string `yaml:"target" json:"target"`
	PoliciesDir string `yaml:"policies_dir" json:"policies_dir"`
}

// DestinationConfig holds optional config for writing governed records to an external data sink.
type DestinationConfig struct {
	// Type is the destination driver name (e.g. "postgres"). Empty string means disabled.
	Type           string                 `yaml:"type"`
	Settings       map[string]string      `yaml:"settings"`
	Auth           pipelinecfg.AuthConfig `yaml:"auth"`
	TLS            pipelinecfg.TLSConfig  `yaml:"tls"`
	CircuitBreaker CircuitBreakerConfig   `yaml:"circuit_breaker"`
}

type Config struct {
	Port               string
	UpstreamURL        string
	AuditFile          string
	AuditMaxSizeMB     int
	AuditMaxAgeDays    int
	AuditFsyncInterval string
	PoliciesDir        string
	IdpURL             string
	Salt               string
	ShutdownTimeoutSec int
	CircuitBreaker     CircuitBreakerConfig
	// Upstreams is the list of static upstream target services.
	Upstreams []UpstreamConfig
	// Destinations is the ordered list of configured data sinks.
	// A single destination is represented as a one-element slice.
	// Zero-length means no destination is configured.
	Destinations []DestinationConfig
}

// yamlConfig represents the structure of auvis.yaml
type yamlConfig struct {
	Gateway struct {
		Port               int                  `yaml:"port"`
		PoliciesDir        string               `yaml:"policies_dir"`
		AuditFile          string               `yaml:"audit_file"`
		AuditMaxSizeMB     int                  `yaml:"audit_max_size_mb"`
		AuditMaxAgeDays    int                  `yaml:"audit_max_age_days"`
		AuditFsyncInterval string               `yaml:"audit_fsync_interval"`
		ShutdownTimeoutSec int                  `yaml:"shutdown_timeout_seconds"`
		IdpURL             string               `yaml:"idp_url"`
		Salt               string               `yaml:"salt"`
		Upstream           string               `yaml:"upstream"`
		Upstreams          []UpstreamConfig     `yaml:"upstreams"`
		CircuitBreaker     CircuitBreakerConfig `yaml:"circuit_breaker"`
		// Legacy singular form — still supported for backward compatibility.
		Destination DestinationConfig `yaml:"destination"`
		// New list form — takes precedence over the singular form when non-empty.
		Destinations []DestinationConfig `yaml:"destinations"`
	} `yaml:"gateway"`
}

func LoadConfig(path string) Config {
	godotenv.Load()

	// Default values
	cfg := Config{
		Port:               "8080",
		AuditFile:          "./audit.jsonl",
		AuditMaxSizeMB:     10,
		AuditMaxAgeDays:    30,
		AuditFsyncInterval: "never",
		PoliciesDir:        "./policies",
		UpstreamURL:        os.Getenv("AUVIS_UPSTREAM"),
		IdpURL:             os.Getenv("AUVIS_IDP_URL"),
		ShutdownTimeoutSec: 30,
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:     5,
			CooldownSeconds: 30,
		},
	}

	// Try reading YAML
	if path != "" {
		b, err := os.ReadFile(path)
		if err == nil {
			var y yamlConfig
			if err := yaml.Unmarshal(b, &y); err != nil {
				log.Printf("failed to parse config %s: %v", path, err)
			} else {
				if y.Gateway.Port != 0 {
					cfg.Port = fmt.Sprintf("%d", y.Gateway.Port)
				}
				if y.Gateway.PoliciesDir != "" {
					cfg.PoliciesDir = y.Gateway.PoliciesDir
				}
				if y.Gateway.AuditFile != "" {
					cfg.AuditFile = y.Gateway.AuditFile
				}
				if y.Gateway.AuditMaxSizeMB != 0 {
					cfg.AuditMaxSizeMB = y.Gateway.AuditMaxSizeMB
				}
				if y.Gateway.AuditMaxAgeDays != 0 {
					cfg.AuditMaxAgeDays = y.Gateway.AuditMaxAgeDays
				}
				if y.Gateway.AuditFsyncInterval != "" {
					cfg.AuditFsyncInterval = y.Gateway.AuditFsyncInterval
				}
				if y.Gateway.ShutdownTimeoutSec != 0 {
					cfg.ShutdownTimeoutSec = y.Gateway.ShutdownTimeoutSec
				}
				if y.Gateway.IdpURL != "" {
					cfg.IdpURL = y.Gateway.IdpURL
				}
				if y.Gateway.Salt != "" {
					cfg.Salt = y.Gateway.Salt
				}
				if y.Gateway.Upstream != "" {
					cfg.UpstreamURL = y.Gateway.Upstream
				}
				if len(y.Gateway.Upstreams) > 0 {
					cfg.Upstreams = y.Gateway.Upstreams
				}
				if y.Gateway.CircuitBreaker.MaxFailures != 0 {
					cfg.CircuitBreaker.MaxFailures = y.Gateway.CircuitBreaker.MaxFailures
				}
				if y.Gateway.CircuitBreaker.CooldownSeconds != 0 {
					cfg.CircuitBreaker.CooldownSeconds = y.Gateway.CircuitBreaker.CooldownSeconds
				}

				// Prefer the new list form; fall back to legacy singular form.
				if len(y.Gateway.Destinations) > 0 {
					cfg.Destinations = y.Gateway.Destinations
				} else if y.Gateway.Destination.Type != "" {
					cfg.Destinations = []DestinationConfig{y.Gateway.Destination}
				}

				// Apply global circuit breaker defaults to destinations that don't override them
				for i := range cfg.Destinations {
					if cfg.Destinations[i].CircuitBreaker.MaxFailures == 0 {
						cfg.Destinations[i].CircuitBreaker.MaxFailures = cfg.CircuitBreaker.MaxFailures
					}
					if cfg.Destinations[i].CircuitBreaker.CooldownSeconds == 0 {
						cfg.Destinations[i].CircuitBreaker.CooldownSeconds = cfg.CircuitBreaker.CooldownSeconds
					}
				}
			}
		}
	}

	// Override with env if present
	if p := os.Getenv("AUVIS_PORT"); p != "" {
		cfg.Port = p
	}
	if a := os.Getenv("AUVIS_AUDIT_FILE"); a != "" {
		cfg.AuditFile = a
	}
	if f := os.Getenv("AUVIS_AUDIT_FSYNC_INTERVAL"); f != "" {
		cfg.AuditFsyncInterval = f
	}

	// Convert single upstream into default element in Upstreams list if Upstreams list is empty
	if len(cfg.Upstreams) == 0 && cfg.UpstreamURL != "" {
		cfg.Upstreams = []UpstreamConfig{
			{
				Name:        "default",
				Prefix:      "/",
				Target:      cfg.UpstreamURL,
				PoliciesDir: cfg.PoliciesDir,
			},
		}
	}

	// Salt loading & environment setting
	salt := os.Getenv("AUVIS_SALT")
	if salt == "" {
		if cfg.Salt != "" {
			salt = cfg.Salt
		} else {
			salt = "default-auvis-salt-value"
		}
	}
	cfg.Salt = salt
	os.Setenv("AUVIS_SALT", salt)

	return cfg
}