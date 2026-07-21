package policy

import (
	"testing"
)

func TestMatchService(t *testing.T) {
	tests := []struct {
		pattern  string
		service  string
		expected bool
	}{
		{"*", "any-service", true},
		{"", "any-service", true},
		{"analytics-*", "analytics-db", true},
		{"analytics-*", "analytics-api", true},
		{"analytics-*", "billing-service", false},
		{"*-service", "billing-service", true},
		{"*-service", "billing-db", false},
		{"billing", "billing", true},
		{"billing", "billing-db", false},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+" vs "+tc.service, func(t *testing.T) {
			res := matchService(tc.pattern, tc.service)
			if res != tc.expected {
				t.Errorf("expected matchService(%q, %q) to be %v, got %v", tc.pattern, tc.service, tc.expected, res)
			}
		})
	}
}

func TestCompileOverlapDetection(t *testing.T) {
	// 1. Two policies matching identical role/resource with overlapping fields should compile (warning only)
	policiesWithOverlap := []Policy{
		{
			Name:     "policy-1",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
			},
		},
		{
			Name:     "policy-2",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "email", Action: ActionRemove}, // same field = overlap
			},
		},
	}

	// Should compile successfully now (overlap is a warning, not an error)
	_, err := compile(policiesWithOverlap)
	if err != nil {
		t.Fatalf("expected successful compilation with overlap warning, got error: %v", err)
	}

	// 2. Two policies matching identical role/resource but NO overlapping fields should also compile
	policiesNoOverlap := []Policy{
		{
			Name:     "policy-1",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
			},
		},
		{
			Name:     "policy-2",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "phone", Action: ActionRemove}, // different field = no overlap
			},
		},
	}

	_, err = compile(policiesNoOverlap)
	if err != nil {
		t.Fatalf("expected successful compilation, got error: %v", err)
	}
}

func TestStoreLookupServiceWildcard(t *testing.T) {
	policies := []Policy{
		{
			Name:     "analyst-analytics-policy",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Services: []string{"analytics-*"},
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
			},
		},
		{
			Name:     "analyst-billing-policy",
			Priority: 200,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Services: []string{"billing-service"},
			},
			Rules: []Rule{
				{Field: "phone", Action: ActionRemove},
			},
		},
	}

	compiled, err := compile(policies)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	store := NewStore()
	store.swap(compiled)

	// 1. Request for "analytics-db" should match "analyst-analytics-policy" (email masked, phone untouched)
	ep1 := store.Lookup("analyst", "/users", "analytics-db", "", "", "read")
	if ep1 == nil {
		t.Fatal("expected ep1 not to be nil")
	}
	if ep1.Rules["email"] != ActionMask {
		t.Errorf("expected 'email' action to be mask, got %q", ep1.Rules["email"])
	}
	if _, hasPhone := ep1.Rules["phone"]; hasPhone {
		t.Error("expected 'phone' rule to be filtered out for analytics service")
	}

	// 2. Request for "billing-service" should match "analyst-billing-policy" (phone removed, email untouched)
	ep2 := store.Lookup("analyst", "/users", "billing-service", "", "", "read")
	if ep2 == nil {
		t.Fatal("expected ep2 not to be nil")
	}
	if ep2.Rules["phone"] != ActionRemove {
		t.Errorf("expected 'phone' action to be remove, got %q", ep2.Rules["phone"])
	}
	if _, hasEmail := ep2.Rules["email"]; hasEmail {
		t.Error("expected 'email' rule to be filtered out for billing service")
	}
}

func TestStoreLookupFieldsConstraint(t *testing.T) {
	policies := []Policy{
		{
			Name:     "analyst-fields-policy",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Fields:   []string{"email", "address"},
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
				{Field: "phone", Action: ActionRemove},
			},
		},
	}

	compiled, err := compile(policies)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	store := NewStore()
	store.swap(compiled)

	ep := store.Lookup("analyst", "/users", "analytics", "", "", "read")
	if ep == nil {
		t.Fatal("expected ep not to be nil")
	}

	// Permitted fields: "email", "address".
	if len(ep.Fields) != 2 || ep.Fields[0] != "email" || ep.Fields[1] != "address" {
		t.Errorf("expected Fields to be [email, address], got %v", ep.Fields)
	}
}

func TestStoreLookupSourceWildcard(t *testing.T) {
	policies := []Policy{
		{
			Name:     "policy-a",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Sources:  []string{`kafka.topic("customer-events")`},
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
			},
		},
		{
			Name:     "policy-b",
			Priority: 200,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Sources:  []string{`kafka.topic('customer-events')`},
			},
			Rules: []Rule{
				{Field: "phone", Action: ActionRemove},
			},
		},
		{
			Name:     "policy-c",
			Priority: 300,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Sources:  []string{`kafka.topic("orders-*")`},
			},
			Rules: []Rule{
				{Field: "ssn", Action: ActionRedact},
			},
		},
		{
			Name:     "policy-d",
			Priority: 400,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
				Sources:  []string{`kafka.*`},
			},
			Rules: []Rule{
				{Field: "name", Action: ActionAllow},
			},
		},
	}

	compiled, err := compile(policies)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	store := NewStore()
	store.swap(compiled)

	// 1. Request matching kafka.customer-events (should match policy-a, policy-b, policy-d)
	ep1 := store.Lookup("analyst", "/users", "some-service", "kafka.customer-events", "", "read")
	if ep1 == nil {
		t.Fatal("expected ep1 not to be nil")
	}
	// Verify rule merge (email masked, phone removed, name allowed)
	if ep1.Rules["email"] != ActionMask {
		t.Errorf("expected email to be mask, got %q", ep1.Rules["email"])
	}
	if ep1.Rules["phone"] != ActionRemove {
		t.Errorf("expected phone to be remove, got %q", ep1.Rules["phone"])
	}
	if ep1.Rules["name"] != ActionAllow {
		t.Errorf("expected name to be allow, got %q", ep1.Rules["name"])
	}
	// Verify ssn is NOT in rules (from policy-c which shouldn't match)
	if _, hasSSN := ep1.Rules["ssn"]; hasSSN {
		t.Error("expected ssn not to be present in ep1")
	}

	// 2. Request matching kafka.orders-created (should match policy-c, policy-d)
	ep2 := store.Lookup("analyst", "/users", "some-service", "kafka.orders-created", "", "read")
	if ep2 == nil {
		t.Fatal("expected ep2 not to be nil")
	}
	// Verify rule merge (ssn redacted, name allowed)
	if ep2.Rules["ssn"] != ActionRedact {
		t.Errorf("expected ssn to be redact, got %q", ep2.Rules["ssn"])
	}
	if ep2.Rules["name"] != ActionAllow {
		t.Errorf("expected name to be allow, got %q", ep2.Rules["name"])
	}
	// Verify email and phone are NOT in rules (from policy-a and policy-b which shouldn't match)
	if _, hasEmail := ep2.Rules["email"]; hasEmail {
		t.Error("expected email not to be present in ep2")
	}
	if _, hasPhone := ep2.Rules["phone"]; hasPhone {
		t.Error("expected phone not to be present in ep2")
	}

	// 3. Request matching non-kafka source (should not match any policy, returning nil)
	ep3 := store.Lookup("analyst", "/users", "some-service", "http.gateway", "", "read")
	if ep3 != nil {
		t.Errorf("expected ep3 to be nil, got %+v", ep3)
	}
}

func TestStoreLookupDestinationWildcard(t *testing.T) {
	policies := []Policy{
		{
			Name:     "policy-a",
			Priority: 100,
			Match: Match{
				Roles:        []string{"analyst"},
				Resource:     "/users",
				Destinations: []string{`vector_db.index("my-index")`},
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
			},
		},
		{
			Name:     "policy-b",
			Priority: 200,
			Match: Match{
				Roles:        []string{"analyst"},
				Resource:     "/users",
				Destinations: []string{`postgres.table('users')`},
			},
			Rules: []Rule{
				{Field: "phone", Action: ActionRemove},
			},
		},
		{
			Name:     "policy-c",
			Priority: 300,
			Match: Match{
				Roles:        []string{"analyst"},
				Resource:     "/users",
				Destinations: []string{`vector_db.index("search-*")`},
			},
			Rules: []Rule{
				{Field: "ssn", Action: ActionRedact},
			},
		},
	}

	compiled, err := compile(policies)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	store := NewStore()
	store.swap(compiled)

	// 1. Request matching vector_db.my-index
	ep1 := store.Lookup("analyst", "/users", "some-service", "some-source", "vector_db.my-index", "read")
	if ep1 == nil {
		t.Fatal("expected ep1 not to be nil")
	}
	if ep1.Rules["email"] != ActionMask {
		t.Errorf("expected email to be mask, got %q", ep1.Rules["email"])
	}
	if _, hasPhone := ep1.Rules["phone"]; hasPhone {
		t.Error("expected phone not to be present in ep1")
	}

	// 2. Request matching vector_db.search-logs
	ep2 := store.Lookup("analyst", "/users", "some-service", "some-source", "vector_db.search-logs", "read")
	if ep2 == nil {
		t.Fatal("expected ep2 not to be nil")
	}
	if ep2.Rules["ssn"] != ActionRedact {
		t.Errorf("expected ssn to be redact, got %q", ep2.Rules["ssn"])
	}

	// 3. Request matching postgres.users
	ep3 := store.Lookup("analyst", "/users", "some-service", "some-source", "postgres.users", "read")
	if ep3 == nil {
		t.Fatal("expected ep3 not to be nil")
	}
	if ep3.Rules["phone"] != ActionRemove {
		t.Errorf("expected phone to be remove, got %q", ep3.Rules["phone"])
	}

	// 4. Unmatched destination
	ep4 := store.Lookup("analyst", "/users", "some-service", "some-source", "redis.cache", "read")
	if ep4 != nil {
		t.Errorf("expected ep4 to be nil, got %+v", ep4)
	}
}

func TestStoreLookupTagMerge(t *testing.T) {
	policies := []Policy{
		{
			Name:     "policy-a-tags",
			Priority: 100,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{FieldsTagged: []string{"@pii"}, Action: ActionMask},
			},
		},
		{
			Name:     "policy-b-tags",
			Priority: 200,
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{FieldsTagged: []string{"@pii"}, Action: ActionRemove}, // Higher priority override
				{FieldsTagged: []string{"@financial"}, Action: ActionRedact},
			},
		},
	}

	compiled, err := compile(policies)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	store := NewStore()
	store.swap(compiled)

	ep := store.Lookup("analyst", "/users", "", "", "", "read")
	if ep == nil {
		t.Fatal("expected ep not to be nil")
	}

	// Verify TaggedRules merged correctly
	if ep.TaggedRules["@pii"] != ActionRemove {
		t.Errorf("expected @pii action to be remove (priority 200), got %v", ep.TaggedRules["@pii"])
	}
	if ep.TaggedRules["@financial"] != ActionRedact {
		t.Errorf("expected @financial action to be redact, got %v", ep.TaggedRules["@financial"])
	}
}

func TestLookupTriggerFiltering(t *testing.T) {
	policies := []Policy{
		{
			Name:     "read-only-policy",
			Priority: 100,
			On:       []string{"read"},
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "email", Action: ActionMask},
			},
		},
		{
			Name:     "ingest-only-policy",
			Priority: 200,
			On:       []string{"ingest"},
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "ssn", Action: ActionRedact},
			},
		},
		{
			Name:     "universal-policy",
			Priority: 300,
			// On is empty — matches all triggers
			Match: Match{
				Roles:    []string{"analyst"},
				Resource: "/users",
			},
			Rules: []Rule{
				{Field: "phone", Action: ActionRemove},
			},
		},
	}

	compiled, err := compile(policies)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	store := NewStore()
	store.swap(compiled)

	// 1. Lookup with trigger "read" should match read-only + universal, not ingest-only
	epRead := store.Lookup("analyst", "/users", "", "", "", "read")
	if epRead == nil {
		t.Fatal("expected epRead not to be nil")
	}
	if epRead.Rules["email"] != ActionMask {
		t.Errorf("expected email to be mask on read trigger, got %q", epRead.Rules["email"])
	}
	if epRead.Rules["phone"] != ActionRemove {
		t.Errorf("expected phone to be remove on read trigger, got %q", epRead.Rules["phone"])
	}
	if _, hasSSN := epRead.Rules["ssn"]; hasSSN {
		t.Error("expected ssn NOT to be present on read trigger")
	}

	// 2. Lookup with trigger "ingest" should match ingest-only + universal, not read-only
	epIngest := store.Lookup("analyst", "/users", "", "", "", "ingest")
	if epIngest == nil {
		t.Fatal("expected epIngest not to be nil")
	}
	if epIngest.Rules["ssn"] != ActionRedact {
		t.Errorf("expected ssn to be redact on ingest trigger, got %q", epIngest.Rules["ssn"])
	}
	if epIngest.Rules["phone"] != ActionRemove {
		t.Errorf("expected phone to be remove on ingest trigger, got %q", epIngest.Rules["phone"])
	}
	if _, hasEmail := epIngest.Rules["email"]; hasEmail {
		t.Error("expected email NOT to be present on ingest trigger")
	}

	// 3. Lookup with empty trigger should match ALL policies
	epAll := store.Lookup("analyst", "/users", "", "", "", "")
	if epAll == nil {
		t.Fatal("expected epAll not to be nil")
	}
	if epAll.Rules["email"] != ActionMask {
		t.Errorf("expected email to be mask on empty trigger, got %q", epAll.Rules["email"])
	}
	if epAll.Rules["ssn"] != ActionRedact {
		t.Errorf("expected ssn to be redact on empty trigger, got %q", epAll.Rules["ssn"])
	}
	if epAll.Rules["phone"] != ActionRemove {
		t.Errorf("expected phone to be remove on empty trigger, got %q", epAll.Rules["phone"])
	}
}
