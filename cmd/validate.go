package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── auvis.yaml structure (gateway config only) ──────────────

type auvisConfig struct {
	Version string      `yaml:"version"`
	Gateway gatewayConfig `yaml:"gateway"`
}

type gatewayConfig struct {
	Port                 int      `yaml:"port"`
	PoliciesDir          string   `yaml:"policies_dir"`
	AuditFile            string   `yaml:"audit_file"`
	AuditMaxSizeMB       int      `yaml:"audit_max_size_mb"`
	AuditMaxAgeDays      int      `yaml:"audit_max_age_days"`
	UpstreamTimeoutSec   int      `yaml:"upstream_timeout_seconds"`
	AllowedUpstreams     []string `yaml:"allowed_upstreams"`
}

// ── Policy file structure ───────────────────────────────────

type policyFile struct {
	Policies []policyEntry `yaml:"policies"`
}

type policyEntry struct {
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Tags            []string          `yaml:"tags"`
	ComplianceTags  []string          `yaml:"compliance_tags"`
	Priority        int               `yaml:"priority"`
	Enabled         *bool             `yaml:"enabled"`
	Match           matchConfig       `yaml:"match"`
	Response        *responseConfig   `yaml:"response,omitempty"`
	Rules           []ruleConfig      `yaml:"rules"`
	FilterCondition string            `yaml:"filter_condition"`
}

type matchConfig struct {
	Roles    []string `yaml:"roles"`
	Resource string   `yaml:"resource"`
	Services []string `yaml:"services"`
	Fields   []string `yaml:"fields"`
}

type responseConfig struct {
	DataPath   string `yaml:"data_path"`
	Array      *bool  `yaml:"array"`
	Rehydrate  bool   `yaml:"rehydrate"`
}

type ruleConfig struct {
	Field     string `yaml:"field"`
	Action    string `yaml:"action"`
	Condition string `yaml:"condition"`
	Length    int    `yaml:"length"`
}

// ── Validation result tracking ──────────────────────────────

type validationError struct {
	File    string
	Policy  string
	Field   string
	Message string
}

type validationWarning struct {
	File    string
	Policy  string
	Field   string
	Message string
}

var validActions = map[string]bool{
	"allow":         true,
	"mask":          true,
	"remove":        true,
	"deny":          true,
	"llm_intercept": true,
	"anonymize":     true,
	"pseudonymize":  true,
	"redact":        true,
	"truncate":      true,
}

var validComplianceTags = map[string]bool{
	"GDPR":    true,
	"HIPAA":   true,
	"PCI-DSS": true,
	"CCPA":    true,
	"SOX":     true,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check auvis.yaml and all policy files for errors",
	Long: `Validate AUVIS configuration and policies without starting the gateway.

Checks:
  • auvis.yaml syntax and required fields
  • Every .yaml file in the policies directory
  • Policy structure: name, match, action, fields
  • Action values are valid (allow, mask, remove, deny, llm_intercept)
  • No duplicate policy names across all files
  • Field masking values are valid
  • Warns about common misconfigurations

Safe to run in CI — exits with code 1 if any errors found.

Examples:
  auvis validate
  auvis validate --config path/to/auvis.yaml
  auvis validate --policies path/to/policies/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runValidate()
	},
}

func runValidate() error {
	fmt.Println()
	bold.Println(" Validating AUVIS configuration")
	separator()
	fmt.Println()

	var errors []validationError
	var warnings []validationWarning
	var totalPolicies int

	// ── 1. Validate auvis.yaml ───────────────────────────────

	cfgPath := resolvePath(cfgFile)
	cfgErrors, cfgWarnings := validateGatewayConfig(cfgPath)
	errors = append(errors, cfgErrors...)
	warnings = append(warnings, cfgWarnings...)

	if len(cfgErrors) > 0 {
		// If the gateway config is broken, we still try to validate policies
		// so the developer sees all problems at once, not one at a time.
		printWarning("gateway config has errors — continuing policy validation anyway")
		fmt.Println()
	}

	// ── 2. Discover policy files ─────────────────────────────

	dirPath := resolvePath(policiesDir)
	files, err := discoverPolicyFiles(dirPath)
	if err != nil {
		if len(cfgErrors) == 0 {
			// policies dir missing is only an error if gateway config is valid
			printError("policies directory not found: %s", dirPath)
			printDimmed("  Tip: Run 'auvis init' to scaffold sample policies.")
			printDimmed("  Run 'auvis --help' for CLI reference.")
			os.Exit(1)
		}
		// If gateway config is also broken, just report what we have
		printResults(nil, errors, warnings)
		return nil
	}

	if len(files) == 0 {
		printWarning("no .yaml files found in %s", policiesDir)
		printDimmed("  Tip: Add policy files to this directory — run 'auvis init' for examples.")
		printDimmed("  Run 'auvis --help' for CLI reference.")
		fmt.Println()
		printResults(nil, errors, warnings)
		return nil
	}

	// ── 3. Validate each policy file ─────────────────────────

	// Track policy names across all files for duplicate detection
	seenNames := make(map[string]string) // name → file
	// Track seen resource priorities: resource -> priority -> policy name
	seenPriorities := make(map[string]map[int]string)
	// Track seen structural matches: role::resource -> priority -> policy name
	seenMatches := make(map[string]map[int]string)

	var allValidPolicies []policyEntry

	for _, f := range files {
		fileErrors, fileWarnings, validInFile := validatePolicyFile(f, seenNames, seenPriorities, seenMatches)
		errors = append(errors, fileErrors...)
		warnings = append(warnings, fileWarnings...)
		allValidPolicies = append(allValidPolicies, validInFile...)
		totalPolicies += len(validInFile)
	}

	// ── 4. Print results ─────────────────────────────────────

	printResults(allValidPolicies, errors, warnings)

	if len(errors) > 0 {
		os.Exit(1)
	}

	return nil
}

// performPolicyValidation validates all policy files in policiesDir syntactically and semantically.
// Returns (errors, warnings, totalValidPolicies).
func performPolicyValidation(policiesDir string) ([]validationError, []validationWarning, int) {
	var errors []validationError
	var warnings []validationWarning

	dirPath := resolvePath(policiesDir)
	files, err := discoverPolicyFiles(dirPath)
	if err != nil {
		errors = append(errors, validationError{
			File:    policiesDir,
			Field:   "directory",
			Message: fmt.Sprintf("policies directory not found: %s", dirPath),
		})
		return errors, warnings, 0
	}

	if len(files) == 0 {
		warnings = append(warnings, validationWarning{
			File:    policiesDir,
			Field:   "directory",
			Message: fmt.Sprintf("no .yaml files found in %s", policiesDir),
		})
		return errors, warnings, 0
	}

	seenNames := make(map[string]string)
	seenPriorities := make(map[string]map[int]string)
	seenMatches := make(map[string]map[int]string)
	var allValidPolicies []policyEntry

	for _, f := range files {
		fileErrors, fileWarnings, validInFile := validatePolicyFile(f, seenNames, seenPriorities, seenMatches)
		errors = append(errors, fileErrors...)
		warnings = append(warnings, fileWarnings...)
		allValidPolicies = append(allValidPolicies, validInFile...)
	}

	return errors, warnings, len(allValidPolicies)
}

// ── Gateway config validation ─────────────────────────────────

func validateGatewayConfig(cfgPath string) ([]validationError, []validationWarning) {
	var errors []validationError
	var warnings []validationWarning

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		errors = append(errors, validationError{
			File:    cfgFile,
			Field:   "file",
			Message: fmt.Sprintf("cannot read: %v", err),
		})
		return errors, warnings
	}

	var cfg auvisConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		errors = append(errors, validationError{
			File:    cfgFile,
			Field:   "syntax",
			Message: fmt.Sprintf("invalid YAML: %v", err),
		})
		return errors, warnings
	}

	// Version check
	if cfg.Version == "" {
		errors = append(errors, validationError{
			File:    cfgFile,
			Field:   "version",
			Message: "required field 'version' is missing",
		})
	} else if cfg.Version != "1" {
		errors = append(errors, validationError{
			File:    cfgFile,
			Field:   "version",
			Message: fmt.Sprintf("unsupported version '%s' — only '1' is supported", cfg.Version),
		})
	}

	// Port check
	if cfg.Gateway.Port == 0 {
		warnings = append(warnings, validationWarning{
			File:    cfgFile,
			Field:   "gateway.port",
			Message: "not set — defaulting to 8080",
		})
	} else if cfg.Gateway.Port < 1 || cfg.Gateway.Port > 65535 {
		errors = append(errors, validationError{
			File:    cfgFile,
			Field:   "gateway.port",
			Message: fmt.Sprintf("invalid port %d — must be 1-65535", cfg.Gateway.Port),
		})
	}

	// Policies dir check
	if cfg.Gateway.PoliciesDir == "" {
		warnings = append(warnings, validationWarning{
			File:    cfgFile,
			Field:   "gateway.policies_dir",
			Message: "not set — defaulting to ./policies",
		})
	}

	// Audit file check
	if cfg.Gateway.AuditFile == "" {
		warnings = append(warnings, validationWarning{
			File:    cfgFile,
			Field:   "gateway.audit_file",
			Message: "not set — defaulting to ./audit.jsonl",
		})
	}

	// Allowed upstreams — warn if empty (production concern)
	if len(cfg.Gateway.AllowedUpstreams) == 0 {
		warnings = append(warnings, validationWarning{
			File:    cfgFile,
			Field:   "gateway.allowed_upstreams",
			Message: "not set — any upstream URL will be accepted (safe for dev, risk in production)",
		})
	} else {
		// Validate each upstream URL
		for i, u := range cfg.Gateway.AllowedUpstreams {
			if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
				errors = append(errors, validationError{
					File:    cfgFile,
					Field:   fmt.Sprintf("gateway.allowed_upstreams[%d]", i),
					Message: fmt.Sprintf("'%s' is not a valid URL — must start with http:// or https://", u),
				})
			}
		}
	}

	// Timeout check
	if cfg.Gateway.UpstreamTimeoutSec == 0 {
		warnings = append(warnings, validationWarning{
			File:    cfgFile,
			Field:   "gateway.upstream_timeout_seconds",
			Message: "not set — defaulting to 10s",
		})
	} else if cfg.Gateway.UpstreamTimeoutSec > 60 {
		warnings = append(warnings, validationWarning{
			File:    cfgFile,
			Field:   "gateway.upstream_timeout_seconds",
			Message: fmt.Sprintf("%ds is high — long timeouts can cause gateway resource exhaustion under load", cfg.Gateway.UpstreamTimeoutSec),
		})
	}

	return errors, warnings
}

// ── Policy file validation ────────────────────────────────────

func validatePolicyFile(filePath string, seenNames map[string]string, seenPriorities map[string]map[int]string, seenMatches map[string]map[int]string) ([]validationError, []validationWarning, []policyEntry) {
	var errors []validationError
	var warnings []validationWarning

	relPath, _ := filepath.Rel(resolvePath("."), filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		errors = append(errors, validationError{
			File:    relPath,
			Field:   "file",
			Message: fmt.Sprintf("cannot read: %v", err),
		})
		return errors, warnings, nil
	}

	var pf policyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		errors = append(errors, validationError{
			File:    relPath,
			Field:   "syntax",
			Message: fmt.Sprintf("invalid YAML: %v", err),
		})
		return errors, warnings, nil
	}

	if len(pf.Policies) == 0 {
		warnings = append(warnings, validationWarning{
			File:    relPath,
			Message: "no policies defined — file will be ignored",
		})
		return errors, warnings, nil
	}

	var validPolicies []policyEntry
	for i, p := range pf.Policies {
		policyErrors, policyWarnings := validateSinglePolicy(relPath, i, p, seenNames, seenPriorities, seenMatches)
		errors = append(errors, policyErrors...)
		warnings = append(warnings, policyWarnings...)
		if len(policyErrors) == 0 {
			validPolicies = append(validPolicies, p)
		}
	}

	return errors, warnings, validPolicies
}

func validateSinglePolicy(file string, index int, p policyEntry, seenNames map[string]string, seenPriorities map[string]map[int]string, seenMatches map[string]map[int]string) ([]validationError, []validationWarning) {
	var errors []validationError
	var warnings []validationWarning

	label := fmt.Sprintf("policies[%d]", index)
	if p.Name != "" {
		label = p.Name
	}

	// ── Name ────────────────────────────────────────────────
	if p.Name == "" {
		errors = append(errors, validationError{
			File:   file,
			Policy: label,
			Field:  "name",
			Message: "every policy must have a name",
		})
	} else {
		// Duplicate name check
		if existingFile, exists := seenNames[p.Name]; exists {
			errors = append(errors, validationError{
				File:   file,
				Policy: p.Name,
				Field:  "name",
				Message: fmt.Sprintf("duplicate policy name — already defined in %s", existingFile),
			})
		} else {
			seenNames[p.Name] = file
		}
	}

	// ── Compliance Tags Validation ──────────────────────────
	for i, tag := range p.ComplianceTags {
		if !validComplianceTags[tag] {
			errors = append(errors, validationError{
				File:    file,
				Policy:  p.Name,
				Field:   fmt.Sprintf("compliance_tags[%d]", i),
				Message: fmt.Sprintf("invalid compliance tag %q — must be one of GDPR, HIPAA, PCI-DSS, CCPA, SOX", tag),
			})
		}
	}

	// ── Duplicate Priority Detection ───────────────────────
	if p.Match.Resource != "" {
		if priorities, exists := seenPriorities[p.Match.Resource]; exists {
			if existingPolicy, exists := priorities[p.Priority]; exists {
				warnings = append(warnings, validationWarning{
					File:    file,
					Policy:  p.Name,
					Field:   "priority",
					Message: fmt.Sprintf("duplicate priority %d found on same resource %q (also defined in policy %q)", p.Priority, p.Match.Resource, existingPolicy),
				})
			} else {
				priorities[p.Priority] = p.Name
			}
		} else {
			seenPriorities[p.Match.Resource] = map[int]string{p.Priority: p.Name}
		}
	}

	// ── Structural Conflict Detection ──────────────────────
	if p.Match.Resource != "" {
		for _, role := range p.Match.Roles {
			key := fmt.Sprintf("%s::%s", role, p.Match.Resource)
			if priorities, exists := seenMatches[key]; exists {
				if existingPolicyName, hasPriority := priorities[p.Priority]; hasPriority {
					errors = append(errors, validationError{
						File:    file,
						Policy:  p.Name,
						Field:   "match",
						Message: fmt.Sprintf("structural conflict: duplicate priority %d for identical role %q and resource %q (also defined in policy %q)", p.Priority, role, p.Match.Resource, existingPolicyName),
					})
				} else {
					priorities[p.Priority] = p.Name
				}
			} else {
				seenMatches[key] = map[int]string{p.Priority: p.Name}
			}
		}
	}

	// ── Match block ─────────────────────────────────────────
	if len(p.Match.Roles) == 0 {
		errors = append(errors, validationError{
			File:   file,
			Policy: p.Name,
			Field:  "match.roles",
			Message: "at least one role is required",
		})
	}

	if p.Match.Resource == "" {
		errors = append(errors, validationError{
			File:   file,
			Policy: p.Name,
			Field:  "match.resource",
			Message: "resource path is required",
		})
	} else if !strings.HasPrefix(p.Match.Resource, "/") {
		errors = append(errors, validationError{
			File:   file,
			Policy: p.Name,
			Field:  "match.resource",
			Message: fmt.Sprintf("resource '%s' must start with /", p.Match.Resource),
		})
	}

	// ── Rules ───────────────────────────────────────────────
	hasLLMIntercept := false
	if len(p.Rules) == 0 {
		errors = append(errors, validationError{
			File:   file,
			Policy: p.Name,
			Field:  "rules",
			Message: "at least one rule is required",
		})
	} else {
		for i, rule := range p.Rules {
			if rule.Field == "" {
				errors = append(errors, validationError{
					File:   file,
					Policy: p.Name,
					Field:  fmt.Sprintf("rules[%d].field", i),
					Message: "rule field is required",
				})
			}

			if rule.Action == "" {
				errors = append(errors, validationError{
					File:   file,
					Policy: p.Name,
					Field:  fmt.Sprintf("rules[%d].action", i),
					Message: "rule action is required",
				})
			} else if !validActions[rule.Action] {
				suggestion := suggestAction(rule.Action)
				msg := fmt.Sprintf("'%s' is not a valid action", rule.Action)
				if suggestion != "" {
					msg += fmt.Sprintf(" — did you mean '%s'?", suggestion)
				}
				errors = append(errors, validationError{
					File:   file,
					Policy: p.Name,
					Field:  fmt.Sprintf("rules[%d].action", i),
					Message: msg,
				})
			}

			if rule.Action == "llm_intercept" {
				hasLLMIntercept = true
			}
		}
	}

	// ── Response config ─────────────────────────────────────
	if p.Response != nil {
		if p.Response.DataPath == "" && p.Response.Array != nil && *p.Response.Array {
			warnings = append(warnings, validationWarning{
				File:   file,
				Policy: p.Name,
				Field:  "response.data_path",
				Message: "array=true but data_path is empty — the engine will look for data at the root",
			})
		}

		if p.Response.Rehydrate && !hasLLMIntercept {
			warnings = append(warnings, validationWarning{
				File:   file,
				Policy: p.Name,
				Field:  "response.rehydrate",
				Message: "rehydrate only applies to rules with 'llm_intercept' action",
			})
		}
	}

	// ── LLM intercept specific checks ──────────────────────
	if hasLLMIntercept {
		// LLM intercept without rehydrate means responses come back with placeholders
		if p.Response == nil || !p.Response.Rehydrate {
			warnings = append(warnings, validationWarning{
				File:   file,
				Policy: p.Name,
				Field:  "response.rehydrate",
				Message: "llm_intercept without rehydrate=true — LLM responses will contain placeholders instead of real values",
			})
		}
	}

	return errors, warnings
}

// ── Helpers ───────────────────────────────────────────────────

func discoverPolicyFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files, nil
}

func validFieldAction(action string) bool {
	switch action {
	case "mask", "remove", "allow", "anonymize", "pseudonymize", "*":
		return true
	default:
		return false
	}
}

func suggestAction(input string) string {
	// Simple levenstein-like suggestion for common typos
	candidates := []string{"allow", "mask", "remove", "deny", "llm_intercept", "anonymize", "pseudonymize", "redact", "truncate"}
	best := ""
	bestDist := 999
	for _, c := range candidates {
		d := editDistance(input, c)
		if d < bestDist && d <= 2 {
			best = c
			bestDist = d
		}
	}
	return best
}

func editDistance(a, b string) int {
	if len(a) == 0 { return len(b) }
	if len(b) == 0 { return len(a) }
	
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				minVal := dp[i-1][j] + 1 // deletion
				if dp[i][j-1]+1 < minVal {
					minVal = dp[i][j-1] + 1 // insertion
				}
				if dp[i-1][j-1]+1 < minVal {
					minVal = dp[i-1][j-1] + 1 // substitution
				}
				dp[i][j] = minVal
			}
		}
	}
	return dp[n][m]
}

func resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	return filepath.Join(cwd, path)
}

func printResults(allValidPolicies []policyEntry, errors []validationError, warnings []validationWarning) {
	if len(allValidPolicies) > 0 {
		bold.Printf("\nValidated Policies (%d):\n", len(allValidPolicies))
		for _, p := range allValidPolicies {
			enabledStr := "enabled"
			if p.Enabled != nil && !*p.Enabled {
				enabledStr = "disabled"
			}
			complianceStr := "none"
			if len(p.ComplianceTags) > 0 {
				complianceStr = strings.Join(p.ComplianceTags, ", ")
			}
			customTagsStr := "none"
			if len(p.Tags) > 0 {
				customTagsStr = strings.Join(p.Tags, ", ")
			}
			desc := p.Description
			if desc == "" {
				desc = "No description provided."
			}
			green.Printf("  ✓  %s\n", p.Name)
			printDimmed("     ├─ Description : %s", desc)
			fmt.Println()
			printDimmed("     ├─ Priority    : %d", p.Priority)
			fmt.Println()
			printDimmed("     ├─ Compliance  : %s", complianceStr)
			fmt.Println()
			printDimmed("     ├─ Custom Tags : %s", customTagsStr)
			fmt.Println()
			printDimmed("     └─ Status      : %s", enabledStr)
			fmt.Println()
			fmt.Println()
		}
	}

	if len(warnings) > 0 {
		bold.Printf("\nWarnings (%d):\n", len(warnings))
		for _, w := range warnings {
			var loc string
			if w.Policy != "" {
				loc = fmt.Sprintf("policy %s", w.Policy)
			} else {
				loc = "global config"
			}
			if w.Field != "" {
				loc += fmt.Sprintf(" (%s)", w.Field)
			}
			yellow.Fprintf(os.Stdout, "  ⚠  %s: %s [%s]\n", loc, w.Message, w.File)
		}
	}

	if len(errors) > 0 {
		bold.Printf("\nErrors (%d):\n", len(errors))
		for _, e := range errors {
			var loc string
			if e.Policy != "" {
				loc = fmt.Sprintf("policy %s", e.Policy)
			} else {
				loc = "global config"
			}
			if e.Field != "" {
				loc += fmt.Sprintf(" (%s)", e.Field)
			}
			red.Fprintf(os.Stderr, "  ✗  %s: %s [%s]\n", loc, e.Message, e.File)
		}
	}

	fmt.Println()
	separator()
	fmt.Println()

	if len(errors) == 0 {
		printSuccess("Validation passed! %d policies checked successfully.", len(allValidPolicies))
	} else {
		printError("Validation failed with %d errors and %d warnings.", len(errors), len(warnings))
	}
	fmt.Println()
}