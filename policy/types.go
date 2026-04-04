package policy

// PolicyFile is the top-level structure of a .yaml policy file.
type PolicyFile struct {
	Policies []Policy `yaml:"policies"`
}

// Policy is a single named policy entry.
type Policy struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"` // human-readable intent
	Tags        []string `yaml:"tags"`         // e.g. [gdpr, pii]
	Match       Match    `yaml:"match"`
	Response    Response `yaml:"response"` // optional — describes the API response shape
	Rules       []Rule   `yaml:"rules"`
}

// Match defines which requests this policy applies to.
type Match struct {
	Roles    []string `yaml:"roles"`
	Resource string   `yaml:"resource"` // prefix match e.g. /users
}

// Response describes where the actual data lives inside the API response.
// Both fields are optional — if omitted, the transform operates on the root.
//
// Example API shapes and what to set:
//
//	[{...}]                      → data_path: ""   array: true  (or omit both)
//	{...}                        → data_path: ""   array: false (or omit both)
//	{"data": [...]}              → data_path: "data"
//	{"result": {"users": [...]}} → data_path: "result.users"
//	{"data": {...}}              → data_path: "data"  array: false
type Response struct {
	// DataPath is the dot-notation path to the records inside the response.
	// Empty string means the records are at the root.
	// Supports gjson dot-notation: "data", "result.users", "_embedded.users"
	DataPath string `yaml:"data_path"`

	// Array hints whether the target is an array of records or a single object.
	// If omitted (nil), the transform auto-detects by inspecting the actual value.
	// Set explicitly when auto-detection might be ambiguous.
	Array *bool `yaml:"array"`
}

// Rule defines what to do with a single field.
type Rule struct {
	Field     string     `yaml:"field"`     // dot-notation: "email", "address.street", "*"
	Action    RuleAction `yaml:"action"`    // mask | remove | allow | deny
	Condition string     `yaml:"condition"` // optional: "record.consent == false"
}

// RuleAction is the set of valid actions a rule can take.
type RuleAction string

const (
	ActionMask   RuleAction = "mask"   // partial redaction e.g. ***@domain.com
	ActionRemove RuleAction = "remove" // delete field entirely
	ActionAllow  RuleAction = "allow"  // pass through untouched
	ActionDeny   RuleAction = "deny"   // block entire response
)

// EffectivePolicy is the compiled, merged ruleset for a specific role+resource.
// This is what the engine works with at request time — never the raw Policy structs.
type EffectivePolicy struct {
	// SourcePolicies are the names of the YAML policies merged into this.
	SourcePolicies []string

	// Response describes the API response shape for this policy.
	// Taken from the last policy merged — conflicts are logged as warnings.
	Response Response

	// Rules is the merged, deduplicated field ruleset.
	// Key is the field path, value is the action to take.
	Rules map[string]RuleAction

	// Deny short-circuits everything — return 403 immediately.
	Deny bool
}