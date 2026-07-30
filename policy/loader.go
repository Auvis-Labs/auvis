package policy

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
	"regexp"
)

// Store holds the compiled effective policies and is safe for concurrent reads.
type Store struct {
	mu       sync.RWMutex
	policies map[string]*EffectivePolicy // key: "role::resource"
}

func NewStore() *Store {
	return &Store{policies: make(map[string]*EffectivePolicy)}
}

// Lookup returns the best-matching effective policy for a role+resource+service+source+destination combination.
// Matching is by longest prefix for resource and wildcard matching for services, sources, and destinations.
// The trigger parameter filters policies by execution context ("read" or "ingest").
// An empty trigger matches all policies. Policies with no triggers defined also match all contexts.
// Returns nil if nothing matches (caller must deny by default).
func (s *Store) Lookup(role, resource, service, source, destination, trigger string) *EffectivePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find all matching compiled policies
	var matches []*EffectivePolicy
	for _, ep := range s.policies {
		// 1. Role match
		roleMatched := false
		for _, r := range ep.Roles {
			if r == role || r == "*" {
				roleMatched = true
				break
			}
		}
		if !roleMatched {
			continue
		}

		// 2. Resource prefix match
		if !strings.HasPrefix(resource, ep.Resource) {
			continue
		}

		// 3. Service match
		if len(ep.Services) > 0 {
			serviceMatched := false
			for _, pattern := range ep.Services {
				if matchService(pattern, service) {
					serviceMatched = true
					break
				}
			}
			if !serviceMatched {
				continue
			}
		}

		// 4. Source match
		if len(ep.Sources) > 0 {
			sourceMatched := false
			for _, pattern := range ep.Sources {
				if matchSource(pattern, source) {
					sourceMatched = true
					break
				}
			}
			if !sourceMatched {
				continue
			}
		}

		// 5. Destination match
		if len(ep.Destinations) > 0 {
			destinationMatched := false
			for _, pattern := range ep.Destinations {
				if matchDestination(pattern, destination) {
					destinationMatched = true
					break
				}
			}
			if !destinationMatched {
				continue
			}
		}

		// 6. Trigger match
		if trigger != "" && len(ep.Triggers) > 0 {
			triggerMatched := false
			for _, t := range ep.Triggers {
				if t == trigger {
					triggerMatched = true
					break
				}
			}
			if !triggerMatched {
				continue
			}
		}

		matches = append(matches, ep)
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort matching policies:
	// 1. Priority ascending (higher priority rules naturally override lower ones)
	// 2. Specificity: wildcard roles "*" are sorted first (lower precedence) than explicit roles (higher precedence)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority < matches[j].Priority
		}
		// Check for wildcard roles "*" in matches[i]
		iHasWildcard := false
		for _, r := range matches[i].Roles {
			if r == "*" {
				iHasWildcard = true
				break
			}
		}
		// Check for wildcard roles "*" in matches[j]
		jHasWildcard := false
		for _, r := range matches[j].Roles {
			if r == "*" {
				jHasWildcard = true
				break
			}
		}
		if iHasWildcard != jHasWildcard {
			// Wildcard role "*" goes first (lower precedence)
			return iHasWildcard
		}
		return false
	})

	// Merge matching policies
	merged := &EffectivePolicy{
		Rules:                make(map[string]RuleAction),
		RuleConditions:       make(map[string]string),
		RuleLengths:          make(map[string]int),
		RuleSeeds:            make(map[string]string),
		TaggedRules:          make(map[string]RuleAction),
		TaggedRuleConditions: make(map[string]string),
		TaggedRuleLengths:    make(map[string]int),
		TaggedRuleSeeds:      make(map[string]string),
	}

	for _, ep := range matches {
		merged.SourcePolicies = append(merged.SourcePolicies, ep.SourcePolicies...)
		
		// Carry Response block — higher priority wins
		if !isResponseEmpty(ep.Response) || isResponseEmpty(merged.Response) {
			merged.Response = ep.Response
		}

		if ep.Deny {
			merged.Deny = true
		} else {
			// If this higher priority or more specific policy matches and does not deny,
			// it means it allows the request to proceed (either fully or with transformations).
			merged.Deny = false
		}

		if ep.FilterCondition != "" {
			merged.FilterCondition = ep.FilterCondition
		}

		if len(ep.ComplianceTags) > 0 {
			merged.ComplianceTags = append(merged.ComplianceTags, ep.ComplianceTags...)
		}

		// Merge rules
		for field, action := range ep.Rules {
			if field == "*" {
				// field="*" is a policy-level directive, not a real field name.
				// ActionDeny  → mark the whole response denied.
				// ActionAllow → allow passthrough (clear deny).
				// Do NOT put "*" into merged.Rules — it's not a sjson-deletable field unless it's ActionLLMIntercept.
				if action == ActionDeny {
					merged.Deny = true
				} else if action == ActionAllow {
					merged.Deny = false
				} else if action == ActionLLMIntercept {
					merged.Rules["*"] = action
				}
				continue
			}
			merged.Rules[field] = action
			merged.RuleConditions[field] = ep.RuleConditions[field]
			if l, ok := ep.RuleLengths[field]; ok {
				merged.RuleLengths[field] = l
			}
			if s, ok := ep.RuleSeeds[field]; ok {
				merged.RuleSeeds[field] = s
			}
		}

		// Merge tag rules
		for tag, action := range ep.TaggedRules {
			merged.TaggedRules[tag] = action
			merged.TaggedRuleConditions[tag] = ep.TaggedRuleConditions[tag]
			if l, ok := ep.TaggedRuleLengths[tag]; ok {
				merged.TaggedRuleLengths[tag] = l
			}
			if s, ok := ep.TaggedRuleSeeds[tag]; ok {
				merged.TaggedRuleSeeds[tag] = s
			}
		}

		// Merge Fields constraint
		if len(ep.Fields) > 0 {
			merged.Fields = append(merged.Fields, ep.Fields...)
		}
	}

	merged.Priority = matches[len(matches)-1].Priority
	merged.ComplianceTags = dedupeSlice(merged.ComplianceTags)

	return merged
}

func matchService(pattern, service string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(service, parts[0]) && strings.HasSuffix(service, parts[1])
		}
		return strings.HasPrefix(service, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == service
}

var kafkaTopicRegex = regexp.MustCompile(`^kafka\.topic\((.*)\)$`)

func matchSource(pattern, source string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	// Try to parse kafka.topic(...) function matchers
	if matches := kafkaTopicRegex.FindStringSubmatch(pattern); len(matches) == 2 {
		topicPattern := strings.Trim(matches[1], `'"`)
		return matchService("kafka."+topicPattern, source)
	}
	// Fallback to generic glob match
	return matchService(pattern, source)
}

var genericDestRegex = regexp.MustCompile(`^([a-zA-Z0-9_]+)\.[a-zA-Z0-9_]+\((.*)\)$`)

func matchDestination(pattern, destination string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	// Try to parse generic generic.func("...") matchers
	if matches := genericDestRegex.FindStringSubmatch(pattern); len(matches) == 3 {
		prefix := matches[1]
		targetPattern := strings.Trim(matches[2], `'"`)
		return matchService(prefix+"."+targetPattern, destination)
	}
	// Fallback to generic glob match
	return matchService(pattern, destination)
}

func (s *Store) swap(next map[string]*EffectivePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies = next
}

// GetPolicies returns a thread-safe copy of all compiled effective policies.
func (s *Store) GetPolicies() map[string]*EffectivePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]*EffectivePolicy, len(s.policies))
	for k, v := range s.policies {
		copy[k] = v
	}
	return copy
}

// LoadDir reads all .yaml/.yml files from dir, compiles them into the store.
// Returns the number of effective policies loaded.
func (s *Store) LoadDir(dir string) (int, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	ymlFiles, _ := filepath.Glob(filepath.Join(dir, "*.yml"))
	files = append(files, ymlFiles...)

	if len(files) == 0 {
		slog.Warn("No YAML policy files found", "directory", dir)
	}

	var allPolicies []Policy
	for _, f := range files {
		parsed, err := parseFile(f)
		if err != nil {
			return 0, fmt.Errorf("parsing %s: %w", f, err)
		}
		slog.Info("Policies loaded successfully", "count", len(parsed), "file", f)
		allPolicies = append(allPolicies, parsed...)
	}

	compiled, err := compile(allPolicies)
	if err != nil {
		return 0, fmt.Errorf("compiling policies: %w", err)
	}

	s.swap(compiled)
	return len(compiled), nil
}

// WatchDir starts a background goroutine that reloads policies on any .yaml change.
func (s *Store) WatchDir(dir string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watching dir %s: %w", dir, err)
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !isYAMLFile(event.Name) {
					continue
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					slog.Info("Policy file change detected — reloading store", "file", event.Name, "operation", event.Op.String())
					n, err := s.LoadDir(dir)
					if err != nil {
						slog.Error("Policy reload failed — retaining active configuration", "error", err)
					} else {
						slog.Info("Policy reload completed successfully", "active_policies", n)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("Policy watcher encountered an error", "error", err)
			}
		}
	}()

	return nil
}

// -------------------------------------------------------------------------------
// Internal helpers
// -------------------------------------------------------------------------------

func parseFile(path string) ([]Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pf PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	for i, p := range pf.Policies {
		if p.Name == "" {
			return nil, fmt.Errorf("policy[%d] missing name", i)
		}
		if p.Match.Resource == "" {
			return nil, fmt.Errorf("policy %q missing match.resource", p.Name)
		}
		if len(p.Match.Roles) == 0 {
			return nil, fmt.Errorf("policy %q missing match.roles", p.Name)
		}
		if len(p.Rules) == 0 {
			return nil, fmt.Errorf("policy %q has no rules", p.Name)
		}
	}

	return pf.Policies, nil
}

// compile merges all raw policies into effective policies keyed by role::resource.
// When two policies share the same role+resource, their rules are merged.
// The Response block from the last matching policy wins (with a warning on conflict).
func compile(policies []Policy) (map[string]*EffectivePolicy, error) {
	// 1. Filter out disabled policies
	var activePolicies []Policy
	for _, p := range policies {
		if p.Enabled != nil && !*p.Enabled {
			slog.Info("Skipping disabled policy configuration", "name", p.Name)
			continue
		}
		activePolicies = append(activePolicies, p)
	}

	// 2. Compile-time overlap detection
	seenMatches := make(map[string][]Policy)
	for _, p := range activePolicies {
		for _, role := range p.Match.Roles {
			key := role + "::" + p.Match.Resource
			seenMatches[key] = append(seenMatches[key], p)
		}
	}
	for key, policies := range seenMatches {
		if len(policies) < 2 {
			continue
		}
		for i := 0; i < len(policies); i++ {
			for j := i + 1; j < len(policies); j++ {
				overlapping := findFieldOverlaps(policies[i], policies[j])
				if len(overlapping) > 0 {
					slog.Warn("Compile-time policy overlap detected — multiple policies act on same fields",
						"policy_a", policies[i].Name,
						"policy_b", policies[j].Name,
						"match", key,
						"overlapping_fields", overlapping,
					)
				}
			}
		}
	}

	// 3. Compile each policy into a standalone EffectivePolicy
	result := make(map[string]*EffectivePolicy)
	for _, p := range activePolicies {
		ep := &EffectivePolicy{
			SourcePolicies:  []string{p.Name},
			ComplianceTags:  p.ComplianceTags,
			Response:        p.Response,
			Rules:                make(map[string]RuleAction),
			RuleConditions:       make(map[string]string),
			RuleLengths:          make(map[string]int),
			RuleSeeds:            make(map[string]string),
			TaggedRules:          make(map[string]RuleAction),
			TaggedRuleConditions: make(map[string]string),
			TaggedRuleLengths:    make(map[string]int),
			TaggedRuleSeeds:      make(map[string]string),
			Services:        p.Match.Services,
			Fields:          p.Match.Fields,
			Sources:         p.Match.Sources,
			Destinations:    p.Match.Destinations,
			Priority:        p.Priority,
			Roles:           p.Match.Roles,
			Resource:        p.Match.Resource,
			FilterCondition: p.FilterCondition,
			Triggers:        p.On,
		}

		for _, rule := range p.Rules {
			if rule.Field == "*" {
				// "*" is a policy-level directive — never store it as a real field in Rules unless it's an LLM interceptor action
				if rule.Action == ActionDeny {
					ep.Deny = true
				}
				if rule.Action == ActionLLMIntercept {
					ep.Rules["*"] = rule.Action
				}
				// ActionAllow on "*" means passthrough — ep.Deny stays false (default).
				continue
			}

			if rule.Field != "" {
				ep.Rules[rule.Field] = rule.Action
				ep.RuleConditions[rule.Field] = rule.Condition
				if rule.Length > 0 {
					ep.RuleLengths[rule.Field] = rule.Length
				}
				if rule.Seed != "" {
					ep.RuleSeeds[rule.Field] = rule.Seed
				}
			}

			if len(rule.FieldsTagged) > 0 {
				for _, tag := range rule.FieldsTagged {
					ep.TaggedRules[tag] = rule.Action
					ep.TaggedRuleConditions[tag] = rule.Condition
					if rule.Length > 0 {
						ep.TaggedRuleLengths[tag] = rule.Length
					}
					if rule.Seed != "" {
						ep.TaggedRuleSeeds[tag] = rule.Seed
					}
				}
			}
		}

		result[p.Name] = ep
	}

	return result, nil
}

func makeKey(role, resource string) string        { return role + "::" + resource }
func SplitKey(key string) (string, string, bool) {
	parts := strings.SplitN(key, "::", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}
func isResponseEmpty(r Response) bool {
	return r.DataPath == "" && r.Array == nil && !r.Rehydrate
}

// findFieldOverlaps returns a list of field names that both policies define rules for.
func findFieldOverlaps(a, b Policy) []string {
	fieldsA := make(map[string]bool)
	for _, r := range a.Rules {
		if r.Field != "" {
			fieldsA[r.Field] = true
		}
	}

	var overlaps []string
	for _, r := range b.Rules {
		if r.Field != "" && fieldsA[r.Field] {
			overlaps = append(overlaps, r.Field)
		}
	}
	return overlaps
}

func dedupeSlice(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}