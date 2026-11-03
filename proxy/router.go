package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/Auvis-Labs/auvis/config"
	"github.com/Auvis-Labs/auvis/policy"
)

// UpstreamRoute holds the resolved target URL, policy store, and config for a specific upstream.
type UpstreamRoute struct {
	Name        string
	Prefix      string
	Header      string
	TargetURL   *url.URL
	TargetRaw   string
	PoliciesDir string
	Store       *policy.Store
}

// UpstreamRouter holds compiled routes for static upstreams and resolves incoming requests in <0.01ms in-memory.
type UpstreamRouter struct {
	routes       []*UpstreamRoute
	defaultRoute *UpstreamRoute
}

// NewUpstreamRouter initializes an UpstreamRouter from configured upstreams and compiles policy stores per upstream.
func NewUpstreamRouter(upstreams []config.UpstreamConfig) (*UpstreamRouter, error) {
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("no upstreams configured")
	}

	var routes []*UpstreamRoute
	var defaultRoute *UpstreamRoute

	for _, u := range upstreams {
		parsedURL, err := url.Parse(u.Target)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream target URL %q for %q: %w", u.Target, u.Name, err)
		}

		store := policy.NewStore()
		if u.PoliciesDir != "" {
			count, err := store.LoadDir(u.PoliciesDir)
			if err != nil {
				slog.Warn("Could not load policy directory for upstream", "upstream", u.Name, "dir", u.PoliciesDir, "error", err)
			} else {
				slog.Info("Loaded policies for upstream", "upstream", u.Name, "count", count, "dir", u.PoliciesDir)
			}
		}

		route := &UpstreamRoute{
			Name:        u.Name,
			Prefix:      u.Prefix,
			Header:      u.Header,
			TargetURL:   parsedURL,
			TargetRaw:   u.Target,
			PoliciesDir: u.PoliciesDir,
			Store:       store,
		}

		routes = append(routes, route)
		if u.Prefix == "/" || u.Name == "default" || defaultRoute == nil {
			defaultRoute = route
		}
	}

	// Sort routes by prefix length descending for longest-prefix matching
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i].Prefix) > len(routes[j].Prefix)
	})

	return &UpstreamRouter{
		routes:       routes,
		defaultRoute: defaultRoute,
	}, nil
}

// Routes returns all configured upstream routes.
func (r *UpstreamRouter) Routes() []*UpstreamRoute {
	return r.routes
}

// DefaultRoute returns the fallback upstream route.
func (r *UpstreamRouter) DefaultRoute() *UpstreamRoute {
	return r.defaultRoute
}

// Resolve finds the matching UpstreamRoute for an incoming HTTP request.
// First checks custom header match, then performs longest path-prefix match.
func (r *UpstreamRouter) Resolve(req *http.Request) *UpstreamRoute {
	// 1. Check header matching
	for _, route := range r.routes {
		if route.Header != "" {
			parts := strings.SplitN(route.Header, ":", 2)
			if len(parts) == 2 {
				hKey := strings.TrimSpace(parts[0])
				hVal := strings.TrimSpace(parts[1])
				if req.Header.Get(hKey) == hVal {
					return route
				}
			} else {
				if req.Header.Get(route.Header) != "" {
					return route
				}
			}
		}
	}

	// 2. Check path prefix matching (sorted longest prefix first)
	path := req.URL.Path
	for _, route := range r.routes {
		if route.Prefix != "" && route.Prefix != "/" {
			if strings.HasPrefix(path, route.Prefix) {
				return route
			}
		}
	}

	// 3. Fallback to default route
	return r.defaultRoute
}
