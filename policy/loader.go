package policy

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Store holds the compiled effective policies and is safe for concurrent reads.
type Store struct {
	mu       sync.RWMutex
	policies map[string]*EffectivePolicy // key: "role::resource"
}

func NewStore() *Store {
	return &Store{policies: make(map[string]*EffectivePolicy)}
}

// Lookup returns the best-matching effective policy for a role+resource pair.
// Matching is by longest prefix — /users matches /users, /users/1, /users/1/posts.
// Returns nil if nothing matches (caller must deny by default).
func (s *Store) Lookup(role, resource string) *EffectivePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	best := (*EffectivePolicy)(nil)
	bestLen := -1

	for key, ep := range s.policies {
		keyRole, keyResource, ok := splitKey(key)
		if !ok {
			continue
		}
		if keyRole == role && strings.HasPrefix(resource, keyResource) {
			if len(keyResource) > bestLen {
				best = ep
				bestLen = len(keyResource)
			}
		}
	}

	return best
}

func (s *Store) swap(next map[string]*EffectivePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies = next
}

// LoadDir reads all .yaml/.yml files from dir, compiles them into the store.
// Returns the number of effective policies loaded.
func (s *Store) LoadDir(dir string) (int, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	ymlFiles, _ := filepath.Glob(filepath.Join(dir, "*.yml"))
	files = append(files, ymlFiles...)

	if len(files) == 0 {
		log.Printf("[POLICY] no .yaml files found in %s", dir)
	}

	var allPolicies []Policy
	for _, f := range files {
		parsed, err := parseFile(f)
		if err != nil {
			return 0, fmt.Errorf("parsing %s: %w", f, err)
		}
		log.Printf("[POLICY] loaded %d policies from %s", len(parsed), f)
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
					log.Printf("[POLICY] change detected: %s (%s) — reloading...", event.Name, event.Op)
					n, err := s.LoadDir(dir)
					if err != nil {
						log.Printf("[POLICY] reload FAILED — keeping previous policies. Error: %v", err)
					} else {
						log.Printf("[POLICY] reload OK — %d effective policies active", n)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[POLICY] watcher error: %v", err)
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
	result := make(map[string]*EffectivePolicy)

	for _, p := range policies {
		for _, role := range p.Match.Roles {
			key := makeKey(role, p.Match.Resource)

			ep, exists := result[key]
			if !exists {
				ep = &EffectivePolicy{
					Rules: make(map[string]RuleAction),
				}
				result[key] = ep
			}

			ep.SourcePolicies = append(ep.SourcePolicies, p.Name)

			// Carry the Response block — warn if two policies disagree on shape
			if exists && ep.Response.DataPath != p.Response.DataPath {
				log.Printf("[POLICY] WARNING: conflicting data_path for role=%s resource=%s — using %q from %s",
					role, p.Match.Resource, p.Response.DataPath, p.Name)
			}
			ep.Response = p.Response

			// Merge rules
			for _, rule := range p.Rules {
				if rule.Action == ActionDeny && rule.Field == "*" {
					ep.Deny = true
					log.Printf("[POLICY] %s — role=%s resource=%s is DENY", p.Name, role, p.Match.Resource)
					continue
				}

				if existing, conflict := ep.Rules[rule.Field]; conflict && existing != rule.Action {
					log.Printf("[POLICY] WARNING conflict on field %q for role=%s resource=%s: %s vs %s (taking %s from %s)",
						rule.Field, role, p.Match.Resource, existing, rule.Action, rule.Action, p.Name)
				}

				ep.Rules[rule.Field] = rule.Action
			}
		}
	}

	return result, nil
}

func makeKey(role, resource string) string        { return role + "::" + resource }
func splitKey(key string) (string, string, bool) {
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