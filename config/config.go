package config

import (
	"fmt"
	"log"
	"os"
	"strings"

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
	Env                string // "development" | "production" (strictly from AUVIS_ENV, default: "development")
	UngovernedPHIPosture string // "warn" | "auto_mask" | "block"
	// Upstreams is the list of static upstream target services.
	Upstreams []UpstreamConfig
	// Destinations is the ordered list of configured data sinks.
	// A single destination is represented as a one-element slice.
	// Zero-length means no destination is configured.
	Destinations []DestinationConfig
}

// yamlConfig represents the structure of auvis.yaml
type yamlConfig struct {
	// Root level options
	PortRoot               int                  `yaml:"port"`
	PoliciesDirRoot        string               `yaml:"policies_dir"`
	AuditFileRoot          string               `yaml:"audit_file"`
	AuditMaxSizeMBRoot     int                  `yaml:"audit_max_size_mb"`
	AuditMaxAgeDaysRoot    int                  `yaml:"audit_max_age_days"`
	AuditFsyncIntervalRoot string               `yaml:"audit_fsync_interval"`
	ShutdownTimeoutSecRoot int                  `yaml:"shutdown_timeout_seconds"`
	IdpURLRoot             string               `yaml:"idp_url"`
	SaltRoot               string               `yaml:"salt"`
	UpstreamRoot           string               `yaml:"upstream"`
	UpstreamsRoot          []UpstreamConfig     `yaml:"upstreams"`
	CircuitBreakerRoot     CircuitBreakerConfig `yaml:"circuit_breaker"`
	DestinationRoot        DestinationConfig    `yaml:"destination"`
	DestinationsRoot       []DestinationConfig  `yaml:"destinations"`
	UngovernedPHIPostureRoot string             `yaml:"ungoverned_phi_posture"`

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
		Destination        DestinationConfig    `yaml:"destination"`
		Destinations       []DestinationConfig  `yaml:"destinations"`
		UngovernedPHIPosture string             `yaml:"ungoverned_phi_posture"`
	} `yaml:"gateway"`
}

func LoadConfig(path string) Config {
	ensureDotEnvExists()
	err := godotenv.Load()
	if err != nil {
		log.Println("WARN: No .env file loaded")
	}

	// Default values
	cfg := Config{
		Port:               "8080",
		AuditFile:          "./audit.jsonl",
		AuditMaxSizeMB:     10,
		AuditMaxAgeDays:    30,
		AuditFsyncInterval: "never",
		PoliciesDir:        "./policies",
		UpstreamURL:        getEnvWithFallback("AUVIS_UPSTREAM_URL", "AUVIS_UPSTREAM"),
		IdpURL:             os.Getenv("AUVIS_IDP_URL"),
		ShutdownTimeoutSec: 30,
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:     5,
			CooldownSeconds: 30,
		},
	}

	var y yamlConfig

	// Try reading YAML
	if path != "" {
		b, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(b, &y); err != nil {
				log.Printf("failed to parse config %s: %v", path, err)
			} else {
				// 1. Port
				port := y.PortRoot
				if port == 0 {
					port = y.Gateway.Port
				}
				if port != 0 {
					cfg.Port = fmt.Sprintf("%d", port)
				}

				// 2. Policies Dir
				policiesDir := y.PoliciesDirRoot
				if policiesDir == "" {
					policiesDir = y.Gateway.PoliciesDir
				}
				if policiesDir != "" {
					cfg.PoliciesDir = policiesDir
				}

				// 3. Audit File
				auditFile := y.AuditFileRoot
				if auditFile == "" {
					auditFile = y.Gateway.AuditFile
				}
				if auditFile != "" {
					cfg.AuditFile = auditFile
				}

				// 4. Audit Max Size
				maxSize := y.AuditMaxSizeMBRoot
				if maxSize == 0 {
					maxSize = y.Gateway.AuditMaxSizeMB
				}
				if maxSize != 0 {
					cfg.AuditMaxSizeMB = maxSize
				}

				// 5. Audit Max Age
				maxAge := y.AuditMaxAgeDaysRoot
				if maxAge == 0 {
					maxAge = y.Gateway.AuditMaxAgeDays
				}
				if maxAge != 0 {
					cfg.AuditMaxAgeDays = maxAge
				}

				// 6. Audit Fsync Interval
				fsync := y.AuditFsyncIntervalRoot
				if fsync == "" {
					fsync = y.Gateway.AuditFsyncInterval
				}
				if fsync != "" {
					cfg.AuditFsyncInterval = fsync
				}

				// 7. Shutdown Timeout
				shutdown := y.ShutdownTimeoutSecRoot
				if shutdown == 0 {
					shutdown = y.Gateway.ShutdownTimeoutSec
				}
				if shutdown != 0 {
					cfg.ShutdownTimeoutSec = shutdown
				}

				// 8. Idp URL
				idp := y.IdpURLRoot
				if idp == "" {
					idp = y.Gateway.IdpURL
				}
				if idp != "" {
					cfg.IdpURL = idp
				}

				// 9. Salt
				saltVal := y.SaltRoot
				if saltVal == "" {
					saltVal = y.Gateway.Salt
				}
				if saltVal != "" {
					cfg.Salt = saltVal
				}

				// 10. Upstream Target
				upstream := y.UpstreamRoot
				if upstream == "" {
					upstream = y.Gateway.Upstream
				}
				if upstream != "" {
					cfg.UpstreamURL = upstream
				}

				// 11. Multi Upstreams
				upstreams := y.UpstreamsRoot
				if len(upstreams) == 0 {
					upstreams = y.Gateway.Upstreams
				}
				if len(upstreams) > 0 {
					cfg.Upstreams = upstreams
				}

				// 12. Circuit Breaker
				cb := y.CircuitBreakerRoot
				if cb.MaxFailures == 0 {
					cb = y.Gateway.CircuitBreaker
				}
				if cb.MaxFailures != 0 {
					cfg.CircuitBreaker.MaxFailures = cb.MaxFailures
				}
				if cb.CooldownSeconds != 0 {
					cfg.CircuitBreaker.CooldownSeconds = cb.CooldownSeconds
				}

				// 13. Destinations
				var dests []DestinationConfig
				if len(y.DestinationsRoot) > 0 {
					dests = y.DestinationsRoot
				} else if y.DestinationRoot.Type != "" {
					dests = []DestinationConfig{y.DestinationRoot}
				} else if len(y.Gateway.Destinations) > 0 {
					dests = y.Gateway.Destinations
				} else if y.Gateway.Destination.Type != "" {
					dests = []DestinationConfig{y.Gateway.Destination}
				}
				if len(dests) > 0 {
					cfg.Destinations = dests
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

	// Environment resolution (STRICTLY AUVIS_ENV)
	env := strings.ToLower(strings.TrimSpace(os.Getenv("AUVIS_ENV")))
	if env == "" {
		env = "development"
	}
	if env != "development" && env != "production" {
		log.Fatalf("FATAL: Invalid AUVIS_ENV='%s'. Must be strictly 'development' or 'production'", env)
	}
	cfg.Env = env

	// Ungoverned PHI Posture resolution
	posture := y.UngovernedPHIPostureRoot
	if posture == "" {
		posture = y.Gateway.UngovernedPHIPosture
	}
	if p := os.Getenv("AUVIS_UNGOVERNED_PHI_POSTURE"); p != "" {
		posture = p
	}
	posture = strings.ToLower(strings.TrimSpace(posture))

	if posture == "" {
		if cfg.Env == "production" {
			posture = "auto_mask"
		} else {
			posture = "warn"
		}
	}
	if posture != "warn" && posture != "auto_mask" && posture != "block" {
		log.Fatalf("FATAL: Invalid ungoverned_phi_posture='%s'. Must be 'warn', 'auto_mask', or 'block'", posture)
	}
	cfg.UngovernedPHIPosture = posture

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
			log.Println("WARN: AUVIS_SALT is empty. Using insecure default salt fallback.")
			salt = "default-auvis-salt-value"
		}
	}
	cfg.Salt = salt
	os.Setenv("AUVIS_SALT", salt)

	return cfg
}

func getEnvWithFallback(preferred, fallback string) string {
	if val := os.Getenv(preferred); val != "" {
		return val
	}
	return os.Getenv(fallback)
}

func ensureDotEnvExists() {
	filePath := ".env"
	defaultVars := map[string]string{
		"AUVIS_SALT":        "dev-mock-salt-sha256-secret-key-109283",
		"AUVIS_GATEWAY_URL": "http://localhost:8090/patients",
		"GEMINI_API_KEY":    "mock_gemini_api_key_placeholder",
		"AUVIS_IDP_URL":     "http://localhost:8080/auth",
	}

	var existingContent string
	bytes, err := os.ReadFile(filePath)
	if err == nil {
		existingContent = string(bytes)
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	for k, v := range defaultVars {
		if !strings.Contains(existingContent, k+"=") {
			_, _ = f.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}
}