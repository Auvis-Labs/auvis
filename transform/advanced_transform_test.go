package transform

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/Auvis-Labs/auvis/policy"
	"github.com/Auvis-Labs/auvis/types"
)

func TestRedactField(t *testing.T) {
	// 1. Test primitive redaction
	record := []byte(`{"name":"Alice","secret":"confidential"}`)
	out, err := redactField(record, "secret")
	if err != nil {
		t.Fatalf("redactField failed: %v", err)
	}
	resSecret := gjson.GetBytes(out, "secret").String()
	if resSecret != "[REDACTED]" {
		t.Errorf("expected secret to be [REDACTED], got %q", resSecret)
	}
	resName := gjson.GetBytes(out, "name").String()
	if resName != "Alice" {
		t.Errorf("expected name to be Alice, got %q", resName)
	}

	// 2. Test nested object redaction (leaf-level targeting)
	recordNested := []byte(`{"user":{"name":"Bob","address":{"city":"Seattle","zip":"98101"}}}`)
	outNested, err := redactField(recordNested, "user.address")
	if err != nil {
		t.Fatalf("redactField failed: %v", err)
	}

	city := gjson.GetBytes(outNested, "user.address.city").String()
	zip := gjson.GetBytes(outNested, "user.address.zip").String()
	name := gjson.GetBytes(outNested, "user.name").String()

	if city != "[REDACTED]" {
		t.Errorf("expected city to be [REDACTED], got %q", city)
	}
	if zip != "[REDACTED]" {
		t.Errorf("expected zip to be [REDACTED], got %q", zip)
	}
	if name != "Bob" {
		t.Errorf("expected name to be Bob, got %q", name)
	}
}

func TestTruncateField(t *testing.T) {
	// 1. Test primitive truncation
	record := []byte(`{"summary":"This is a very long text that needs to be truncated","count":42}`)
	out, err := truncateField(record, "summary", 10)
	if err != nil {
		t.Fatalf("truncateField failed: %v", err)
	}
	resSummary := gjson.GetBytes(out, "summary").String()
	if resSummary != "This is a " { // "This is a " is exactly 10 characters
		t.Errorf("expected summary to be truncated to 'This is a ', got %q", resSummary)
	}
	resCount := gjson.GetBytes(out, "count").Int()
	if resCount != 42 {
		t.Errorf("expected count to be untouched, got %d", resCount)
	}

	// 2. Test UTF-8 multi-byte rune safety
	recordUTF8 := []byte(`{"emoji":"👋👾👽🚀🛰️"}`)
	outUTF8, err := truncateField(recordUTF8, "emoji", 3)
	if err != nil {
		t.Fatalf("truncateField failed: %v", err)
	}
	resEmoji := gjson.GetBytes(outUTF8, "emoji").String()
	if resEmoji != "👋👾👽" {
		t.Errorf("expected emoji to be truncated to 3 runes '👋👾👽', got %q", resEmoji)
	}

	// 3. Test nested object truncation
	recordNested := []byte(`{"logs":{"info":"Standard log entry info","err":"Critical error stacktrace"}}`)
	outNested, err := truncateField(recordNested, "logs", 8)
	if err != nil {
		t.Fatalf("truncateField failed: %v", err)
	}
	resInfo := gjson.GetBytes(outNested, "logs.info").String()
	resErr := gjson.GetBytes(outNested, "logs.err").String()

	if resInfo != "Standard" { // 8 characters
		t.Errorf("expected info to be truncated to 'Standard', got %q", resInfo)
	}
	if resErr != "Critical" { // 8 characters
		t.Errorf("expected err to be truncated to 'Critical', got %q", resErr)
	}
}

func TestTransformRecordActionSwitch(t *testing.T) {
	ep := &policy.EffectivePolicy{
		SourcePolicies: []string{"test-policy"},
		Rules: map[string]policy.RuleAction{
			"secret":    policy.ActionRedact,
			"biography": policy.ActionTruncate,
		},
		RuleLengths: map[string]int{
			"biography": 15,
		},
	}

	record := []byte(`{"secret":"password123","biography":"Born in a cabin in the woods of Kentucky","age":30}`)
	reqCtx := types.RequestContext{
		RequestID: "test-req-123",
	}

	var conds []string
	out, disp, err := transformRecord(record, reqCtx, ep, &conds)
	if err != nil {
		t.Fatalf("transformRecord failed: %v", err)
	}

	secret := gjson.GetBytes(out, "secret").String()
	bio := gjson.GetBytes(out, "biography").String()
	age := gjson.GetBytes(out, "age").Int()

	if secret != "[REDACTED]" {
		t.Errorf("expected secret to be [REDACTED], got %q", secret)
	}
	if bio != "Born in a cabin" { // 15 characters
		t.Errorf("expected biography to be 'Born in a cabin', got %q", bio)
	}
	if age != 30 {
		t.Errorf("expected age to be 30, got %d", age)
	}

	// Verify both were marked as masked
	if len(disp.masked) != 2 {
		t.Errorf("expected 2 masked fields, got %v", disp.masked)
	}
}

func TestRowLevelFiltering(t *testing.T) {
	// Setup request context
	reqCtx := types.RequestContext{
		RequestID: "test-req-123",
		UserRole:  "developer",
		UserAttributes: map[string]string{
			"region": "US",
		},
	}

	// 1. Basic filter (record.allowed == true)
	ep := &policy.EffectivePolicy{
		SourcePolicies:  []string{"row-filter-policy"},
		FilterCondition: "record.allowed == true",
		Rules:           make(map[string]policy.RuleAction),
	}

	arrayData := gjson.Parse(`[
		{"id": 1, "allowed": true, "name": "Alice"},
		{"id": 2, "allowed": false, "name": "Bob"},
		{"id": 3, "allowed": true, "name": "Charlie"}
	]`)

	var conds []string
	out, _, _, _, _, rowsFiltered, _, err := transformArray(arrayData, reqCtx, ep, &conds)
	if err != nil {
		t.Fatalf("transformArray failed: %v", err)
	}

	if len(conds) != 3 {
		t.Errorf("expected 3 conditions evaluated, got %d: %v", len(conds), conds)
	}
	expectedCond := "filter: record.allowed == true (true)"
	if conds[0] != expectedCond {
		t.Errorf("expected first condition to be %q, got %q", expectedCond, conds[0])
	}

	if rowsFiltered != 1 {
		t.Errorf("expected 1 row filtered, got %d", rowsFiltered)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 records, got %d", len(parsed))
	}
	if parsed[0]["name"] != "Alice" || parsed[1]["name"] != "Charlie" {
		t.Errorf("unexpected output records: %v", parsed)
	}

	// 2. Complex dynamic filter involving request context (record.region == user.region)
	epDynamic := &policy.EffectivePolicy{
		SourcePolicies:  []string{"row-filter-policy-dynamic"},
		FilterCondition: "record.region == user.region",
		Rules:           make(map[string]policy.RuleAction),
	}

	arrayDataDynamic := gjson.Parse(`[
		{"id": 1, "region": "US", "name": "Alice"},
		{"id": 2, "region": "EU", "name": "Bob"},
		{"id": 3, "region": "US", "name": "Charlie"}
	]`)

	var condsDyn []string
	outDyn, _, _, _, _, rowsFilteredDyn, _, err := transformArray(arrayDataDynamic, reqCtx, epDynamic, &condsDyn)
	if err != nil {
		t.Fatalf("transformArray failed: %v", err)
	}

	if rowsFilteredDyn != 1 {
		t.Errorf("expected 1 row filtered, got %d", rowsFilteredDyn)
	}

	var parsedDyn []map[string]interface{}
	if err := json.Unmarshal(outDyn, &parsedDyn); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(parsedDyn) != 2 {
		t.Errorf("expected 2 records, got %d", len(parsedDyn))
	}

	// 3. Fail-closed on Indeterminate state (missing record field in condition)
	epFailClosed := &policy.EffectivePolicy{
		SourcePolicies:  []string{"row-filter-policy-failclosed"},
		FilterCondition: "record.nonexistent == 'value'",
		Rules:           make(map[string]policy.RuleAction),
	}

	var condsFC []string
	outFC, _, _, _, _, rowsFilteredFC, _, err := transformArray(arrayData, reqCtx, epFailClosed, &condsFC)
	if err != nil {
		t.Fatalf("transformArray failed: %v", err)
	}

	// Since "nonexistent" is missing, every row evaluates to EvalIndeterminate and is filtered out (fail-closed)
	if rowsFilteredFC != 3 {
		t.Errorf("expected 3 rows filtered, got %d", rowsFilteredFC)
	}

	var parsedFC []map[string]interface{}
	if err := json.Unmarshal(outFC, &parsedFC); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(parsedFC) != 0 {
		t.Errorf("expected 0 records after fail-closed filtering, got %d", len(parsedFC))
	}
}

func TestTagBasedFieldMatching(t *testing.T) {
	ep := &policy.EffectivePolicy{
		SourcePolicies: []string{"test-tag-policy"},
		Rules: map[string]policy.RuleAction{
			"explicit_field": policy.ActionRedact,
		},
		TaggedRules: map[string]policy.RuleAction{
			"@pii":       policy.ActionMask,
			"@financial": policy.ActionRemove, // Remove has higher priority than Mask
		},
	}

	record := []byte(`{"explicit_field":"secret","email":"alice@example.com","credit_card":"1234-5678","phone":"555-0198"}`)
	reqCtx := types.RequestContext{
		RequestID: "test-req-123",
		FieldTags: map[string][]string{
			"email":       {"@pii"},
			"credit_card": {"@pii", "@financial"}, // Conflict: @financial (Remove) should win over @pii (Mask)
			// phone has no tags
		},
	}

	var conds []string
	out, disp, err := transformRecord(record, reqCtx, ep, &conds)
	if err != nil {
		t.Fatalf("transformRecord failed: %v", err)
	}

	// 1. explicit_field should be redacted
	if gjson.GetBytes(out, "explicit_field").String() != "[REDACTED]" {
		t.Errorf("expected explicit_field to be [REDACTED], got %s", gjson.GetBytes(out, "explicit_field").String())
	}

	// 2. email should be masked (has @pii)
	email := gjson.GetBytes(out, "email").String()
	if email == "alice@example.com" || email == "" {
		t.Errorf("expected email to be masked, got %s", email)
	}

	// 3. credit_card should be removed (has @financial which overrides @pii)
	if gjson.GetBytes(out, "credit_card").Exists() {
		t.Errorf("expected credit_card to be removed, but it exists: %s", gjson.GetBytes(out, "credit_card").String())
	}

	// 4. phone should be untouched (no tags, no explicit rules)
	if gjson.GetBytes(out, "phone").String() != "555-0198" {
		t.Errorf("expected phone to be untouched, got %s", gjson.GetBytes(out, "phone").String())
	}

	// Check dispositions
	if len(disp.masked) != 2 || !((disp.masked[0] == "explicit_field" && disp.masked[1] == "email") || (disp.masked[0] == "email" && disp.masked[1] == "explicit_field")) {
		t.Errorf("expected masked to be [explicit_field, email] in any order, got %v", disp.masked)
	}
	if len(disp.removed) != 1 || disp.removed[0] != "credit_card" {
		t.Errorf("expected removed to be [credit_card], got %v", disp.removed)
	}
}

func TestFieldLevelConditionWiring(t *testing.T) {
	// Setup request context
	reqCtx := types.RequestContext{
		RequestID: "test-req-123",
		UserRole:  "analyst",
	}

	// Define a policy containing a conditional rule:
	// If record.restricted == true, email should be masked.
	ep := &policy.EffectivePolicy{
		SourcePolicies: []string{"conditional-mask-policy"},
		Rules: map[string]policy.RuleAction{
			"email": policy.ActionMask,
		},
		RuleConditions: map[string]string{
			"email": "record.restricted == true",
		},
	}

	// Case 1: condition evaluates to true -> email should be masked
	recordTrue := []byte(`{"email":"alice@example.com","restricted":true}`)
	var condsTrue []string
	outTrue, dispTrue, err := transformRecord(recordTrue, reqCtx, ep, &condsTrue)
	if err != nil {
		t.Fatalf("transformRecord failed: %v", err)
	}
	emailTrue := gjson.GetBytes(outTrue, "email").String()
	if emailTrue == "alice@example.com" {
		t.Errorf("expected email to be masked when condition is true, got: %s", emailTrue)
	}
	if len(condsTrue) != 1 || condsTrue[0] != "field.email: record.restricted == true (true)" {
		t.Errorf("expected condition log to be true, got: %v", condsTrue)
	}
	if len(dispTrue.masked) != 1 || dispTrue.masked[0] != "email" {
		t.Errorf("expected email in masked disposition, got: %v", dispTrue.masked)
	}

	// Case 2: condition evaluates to false -> email should remain untouched
	recordFalse := []byte(`{"email":"alice@example.com","restricted":false}`)
	var condsFalse []string
	outFalse, dispFalse, err := transformRecord(recordFalse, reqCtx, ep, &condsFalse)
	if err != nil {
		t.Fatalf("transformRecord failed: %v", err)
	}
	emailFalse := gjson.GetBytes(outFalse, "email").String()
	if emailFalse != "alice@example.com" {
		t.Errorf("expected email to be untouched when condition is false, got: %s", emailFalse)
	}
	if len(condsFalse) != 1 || condsFalse[0] != "field.email: record.restricted == true (false)" {
		t.Errorf("expected condition log to be false, got: %v", condsFalse)
	}
	if len(dispFalse.masked) != 0 {
		t.Errorf("expected no masked disposition, got: %v", dispFalse.masked)
	}
}

