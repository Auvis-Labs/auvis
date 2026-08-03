package policy

// PolicyFile is the top-level structure of a .yaml policy file.
type PolicyFile struct {
	Policies []Policy `yaml:"policies"`
}

// Policy is a single named policy entry.
type Policy struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"` // human-readable intent
	Tags            []string `yaml:"tags"`         // e.g. [gdpr, pii]
	ComplianceTags  []string `yaml:"compliance_tags"`
	Priority        int      `yaml:"priority"`
	Enabled         *bool    `yaml:"enabled"`
	On              []string `yaml:"on"` // trigger context: ["read"], ["ingest"], or both. Empty means all.
	Match           Match    `yaml:"match"`
	Response        Response `yaml:"response"` // optional — describes the API response shape
	Rules           []Rule   `yaml:"rules"`
	FilterCondition string   `yaml:"filter_condition"` // optional: row-level filtering condition
	UngovernedPHIPosture string `yaml:"ungoverned_phi_posture"` // optional posture override: "warn" | "auto_mask" | "block"
}

// Match defines which requests this policy applies to.
type Match struct {
	Roles    []string `yaml:"roles"`
	Resource string   `yaml:"resource"` // prefix match e.g. /users
	Services []string `yaml:"services"` // wildcard service list e.g. ["analytics-*"]
	Fields       []string `yaml:"fields"`       // fields list
	Sources      []string `yaml:"sources"`      // ingestion source patterns list
	Destinations []string `yaml:"destinations"` // egress destination patterns list
}

// Response describes where the actual data lives inside the API response.
// Both fields are optional — if omitted, the transform operates on the root.
// Set data_path: "auto" (or "detect") to auto-detect common envelope wrappers ("data", "result", "payload", etc.).
//
// Example API shapes and what to set:
//
//	[{...}]                      → data_path: ""   array: true  (or omit both)
//	{...}                        → data_path: ""   array: false (or omit both)
//	{"data": [...]}              → data_path: "data"
//	{"result": {"users": [...]}} → data_path: "result.users"
//	{"data": {...}}              → data_path: "data"  array: false
//	{"payload": {...}}           → data_path: "auto"
type Response struct {
	// DataPath is the dot-notation path to the records inside the response.
	// Empty string means the records are at the root.
	// Supports gjson dot-notation: "data", "result.users", "_embedded.users"
	// Set to "auto" or "detect" to automatically inspect standard envelope wrappers.
	DataPath string `yaml:"data_path"`

	// Array hints whether the target is an array of records or a single object.
	// If omitted (nil), the transform auto-detects by inspecting the actual value.
	// Set explicitly when auto-detection might be ambiguous.
	Array *bool `yaml:"array"`

	// Rehydrate determines whether masked placeholders in LLM responses are rehydrated.
	Rehydrate bool `yaml:"rehydrate"`
}

// Rule defines what to do with a single field.
type Rule struct {
	Field        string     `yaml:"field"`         // dot-notation: "email", "address.street", "*"
	FieldsTagged []string   `yaml:"fields_tagged"` // tag list: ["@pii", "@financial"]
	Action       RuleAction `yaml:"action"`        // mask | remove | allow | deny | llm_intercept | redact | truncate
	Condition string     `yaml:"condition"` // optional: "record.consent == false"
	Length    int        `yaml:"length"`    // optional: used for truncate length
	Seed      string     `yaml:"seed"`      // optional: per-rule pseudonymization seed override
}

// RuleAction is the set of valid actions a rule can take.
type RuleAction string

const (
	ActionMask         RuleAction = "mask"          // partial redaction e.g. ***@domain.com
	ActionRemove       RuleAction = "remove"        // delete field entirely
	ActionAllow        RuleAction = "allow"         // pass through untouched
	ActionDeny         RuleAction = "deny"          // block entire response
	ActionLLMIntercept RuleAction = "llm_intercept" // scan and mask request + rehydrate response
	ActionAnonymize    RuleAction = "anonymize"    // one-way SHA-256 hash with salt
	ActionPseudonymize RuleAction = "pseudonymize" // one-way deterministic format-preserving HMAC with salt
	ActionRedact       RuleAction = "redact"       // replace with [REDACTED]
	ActionTruncate     RuleAction = "truncate"     // keep first N characters
)

// EffectivePolicy is the compiled, merged ruleset for a specific role+resource.
// This is what the engine works with at request time — never the raw Policy structs.
type EffectivePolicy struct {
	// SourcePolicies are the names of the YAML policies merged into this.
	SourcePolicies []string

	// ComplianceTags is the merged list of compliance tags from all source policies.
	ComplianceTags []string

	// Response describes the API response shape for this policy.
	// Taken from the last policy merged — conflicts are logged as warnings.
	Response Response

	// Rules is the merged, deduplicated field ruleset.
	// Key is the field path, value is the action to take.
	Rules map[string]RuleAction

	// RuleConditions tracks the condition expression for each field.
	// Key is the field path, value is the optional condition string.
	RuleConditions map[string]string

	// Deny short-circuits everything — return 403 immediately.
	Deny bool

	// Services matches the wildcard services constraint for this policy.
	Services []string

	// Fields matches the permitted fields constraint for this policy.
	Fields []string

	// Sources matches the operational ingestion sources for this policy.
	Sources []string

	// Destinations matches the egress destinations for this policy.
	Destinations []string

	// Priority is the policy priority used for sorting and merging at runtime.
	Priority int

	// Roles matches which roles this policy applies to.
	Roles []string

	// Resource matches the resource path prefix.
	Resource string

	// FilterCondition is the compiled row-level filtering condition.
	FilterCondition string

	// RuleLengths tracks the truncation length for each field.
	// Key is the field path, value is the length.
	RuleLengths map[string]int

	// RuleSeeds tracks the pseudonymization seed for each field.
	// Key is the field path, value is the seed override string.
	RuleSeeds map[string]string

	// TaggedRules is the compiled field tag ruleset.
	// Key is the tag, value is the action to take.
	TaggedRules map[string]RuleAction

	// TaggedRuleConditions tracks the condition expression for each tag.
	TaggedRuleConditions map[string]string

	// TaggedRuleLengths tracks the truncation length for each tag.
	TaggedRuleLengths map[string]int

	// TaggedRuleSeeds tracks the pseudonymization seed for each tag.
	TaggedRuleSeeds map[string]string

	// Triggers lists the execution contexts this policy applies to (e.g. "read", "ingest").
	// Empty means the policy applies to all triggers.
	Triggers []string

	// UngovernedPHIPosture is the optional policy-level posture override.
	UngovernedPHIPosture string
}