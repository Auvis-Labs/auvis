package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

// Default auvis.yaml content — written to the project root on `auvis init`.
// Intentionally verbose with comments — first-time developers read this file.
const defaultAuvisYAML = `# auvis.yaml — AUVIS gateway configuration
# Documentation: https://auvis.dev/docs/configuration
#
# This file configures the AUVIS gateway for this project.
# Define WHERE the gateway runs and HOW it behaves.
# Policies live in ./policies/ — one file per domain.

version: "1"

gateway:
  # Port the AUVIS gateway listens on.
  # Your application routes requests through this port.
  port: 8080
  upstream: "http://localhost:8000"

  # Static Multi-Upstream Microservice Routing (Optional list form)
  # Allows one gateway to route to multiple backend microservices based on path prefix or header:
  # upstreams:
  #   - name: "patient-service"
  #     prefix: "/api/v1/patients"
  #     target: "http://localhost:8081"
  #     policies_dir: "./policies/patients"
  #   - name: "billing-service"
  #     prefix: "/api/v1/billing"
  #     target: "http://localhost:8083"
  #     policies_dir: "./policies/billing"

  # Directory containing your policy YAML files.
  # All .yaml files in this directory are loaded and merged at startup.
  # The gateway watches this directory — edits apply instantly, no restart needed.
  policies_dir: ./policies

  # Audit log file path.
  # Every request is logged here as newline-delimited JSON (JSONL).
  # Use 'auvis logs' to query it, or point any log aggregator at this file.
  audit_file: ./audit.jsonl

  # Maximum audit log file size before rotation (in MB).
  audit_max_size_mb: 100

  # Maximum duration in seconds to wait for in-flight request draining during SIGTERM shutdown.
  shutdown_timeout_seconds: 30

  # Upstream request timeout in seconds.
  # Requests to your upstream APIs that exceed this are cancelled.
  upstream_timeout_seconds: 10

  # (Optional) Circuit Breaker configuration defaults for destination write protection.
  # Trips to Open state after max_failures consecutive errors, fast-failing all writes during cooldown_seconds.
  circuit_breaker:
    max_failures: 5
    cooldown_seconds: 30

  # (Optional) Restrict which upstream hosts AUVIS is allowed to forward to.
  # Leave empty to allow any upstream (default for development).
  # Strongly recommended to populate in production.


  # (Optional) Destination — synchronously write governed records to a data sink after each response.
  # When configured, every governed response is written to the destination before being returned to the client.
  # Writes are synchronous and fail-closed: if the write fails, the client receives a 500.
  # destination:
  #   type: pinecone
  #   settings:
  #     index_host: my-index-abcd.svc.environment.pinecone.io
  #     vector_field: embedding  # the field in response data containing the float array
  #   auth:
  #     password: <pinecone_api_key>
`

// Default .env content — created on `auvis init` if missing.
const defaultDotEnv = `# .env — AUVIS Gateway Environment Variables
# Documentation: https://auvis.dev/docs/configuration

# AUVIS_UPSTREAM defines the target backend API or microservice URL that AUVIS proxies requests to.
# In development, set this to your local backend server (e.g. http://localhost:8000).
# In production, set this to your upstream cluster endpoint or internal load balancer.
AUVIS_UPSTREAM=http://localhost:8000
`

// Default policy directory structure — created on `auvis init`.
// Each file is a domain-scoped template the developer fills in.
var defaultPolicies = map[string]string{
	"example.yaml": `# policies/example.yaml
# ─────────────────────────────────────────────────────────────────
# Example policies — rename this file and adapt to your use case.
# All .yaml files in this directory are loaded and merged at startup.
#
# Supported actions:
#   allow          — pass through with no transformation
#   mask           — replace field value with a masked version
#   remove         — delete the field from the response entirely
#   deny           — block access entirely, return 403
#   llm_intercept  — scan and mask PII before forwarding to an LLM API
#
# Documentation: https://auvis.dev/docs/policies
# ─────────────────────────────────────────────────────────────────

policies:

  # ── Example 1: Admin gets full access ──────────────────────────
  - name: example-admin-full-access
    match:
      roles: [admin]
      resource: /users
    rules:
      - field: "*"
        action: allow
    response:
      data_path: "data"
      array: true

  # ── Example 2: Analyst gets masked data ────────────────────────
  - name: example-analyst-masked
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: mask
      - field: "ssn"
        action: remove
      - field: "phone"
        action: mask
      - field: "date_of_birth"
        action: mask
    response:
      data_path: "data"
      array: true

  # ── Example 3: LLM PII interception ────────────────────────────
  # - name: strip-pii-before-llm
  #   match:
  #     roles: [developer]
  #     resource: /v1/chat/completions
  #   rules:
  #     - field: "*"
  #       action: llm_intercept
  #   response:
  #     rehydrate: true
`,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold auvis.yaml and policies/ in the current directory",
	Long: `Initialize AUVIS in the current project directory.

Creates:
  auvis.yaml      gateway configuration file
  policies/       directory for your policy files
  policies/example.yaml   example policy file to get started

Safe to run in any directory. Will not overwrite existing files.

Examples:
  auvis init
  auvis init --config path/to/auvis.yaml`,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	fmt.Println()
	bold.Println("  Initializing AUVIS in current directory")
	separator()

	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Suffix = "  checking environment..."
	s.Start()
	time.Sleep(300 * time.Millisecond)
	s.Stop()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine current directory: %w", err)
	}

	var created []string
	var skipped []string

	// ── 1. Write auvis.yaml ───────────────────────────────────────
	cfgPath := filepath.Join(cwd, cfgFile)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, []byte(defaultAuvisYAML), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", cfgFile, err)
		}
		created = append(created, cfgFile)
	} else {
		skipped = append(skipped, cfgFile)
	}

	// ── 1b. Write .env ─────────────────────────────────────────────
	envPath := filepath.Join(cwd, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if err := os.WriteFile(envPath, []byte(defaultDotEnv), 0644); err != nil {
			return fmt.Errorf("writing .env: %w", err)
		}
		created = append(created, ".env")
	} else {
		skipped = append(skipped, ".env")
	}

	// ── 2. Create policies/ directory ────────────────────────────
	policiesPath := filepath.Join(cwd, policiesDir)
	if _, err := os.Stat(policiesPath); os.IsNotExist(err) {
		if err := os.MkdirAll(policiesPath, 0755); err != nil {
			return fmt.Errorf("creating policies directory: %w", err)
		}
		created = append(created, policiesDir+"/")
	} else {
		skipped = append(skipped, policiesDir+"/")
	}

	// ── 3. Write default policy files ────────────────────────────
	for filename, content := range defaultPolicies {
		policyPath := filepath.Join(policiesPath, filename)
		if _, err := os.Stat(policyPath); os.IsNotExist(err) {
			if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing policy file %s: %w", filename, err)
			}
			created = append(created, filepath.Join(policiesDir, filename))
		} else {
			skipped = append(skipped, filepath.Join(policiesDir, filename))
		}
	}

	// ── 4. Print results ──────────────────────────────────────────
	fmt.Println()

	if len(created) > 0 {
		bold.Println("  Created:")
		for _, f := range created {
			printSuccess("%s", f)
		}
	}

	if len(skipped) > 0 {
		fmt.Println()
		bold.Println("  Skipped (already exists):")
		for _, f := range skipped {
			printWarning("%s", f)
		}
	}

	separator()
	fmt.Println()
	bold.Println("  Next steps:")
	fmt.Println()
	printInfo("Define your policies in %s", policiesDir+"/")
	printInfo("Start the gateway:")
	fmt.Println()
	cyan.Println("    auvis dev")
	fmt.Println()
	printDimmed("Documentation: https://auvis.dev/docs")
	fmt.Println()

	return nil
}
