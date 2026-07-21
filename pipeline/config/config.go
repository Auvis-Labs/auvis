package pipeline

// AuthType represents the method used to authenticate with an external data source or destination.
type AuthType string

const (
	AuthTypeAPIKey  AuthType = "api_key"
	AuthTypeBasic   AuthType = "basic"
	AuthTypeSASLSSL AuthType = "sasl_ssl"
	AuthTypeOAuth2  AuthType = "oauth2"
	AuthTypeIAMRole AuthType = "iam_role"
)

// AuthConfig aggregates parameters required by all supported pipeline authentication schemes.
// A connector (Source/Destination) picks and initializes its authentication using the configured Type.
type AuthConfig struct {
	// Type specifies which authentication style is active: api_key, basic, sasl_ssl, oauth2, or iam_role.
	Type AuthType `yaml:"type" json:"type"`

	// APIKey stores the raw token or secret used for simple token-based checks (api_key auth).
	APIKey string `yaml:"api_key" json:"api_key"`

	// Username is the user login name (basic / sasl_ssl auth).
	Username string `yaml:"username" json:"username"`

	// Password is the matching password or credentials secret (basic / sasl_ssl auth).
	Password string `yaml:"password" json:"password"`

	// SASLMechanism defines the secure channel challenge-response algorithm (sasl_ssl auth).
	// Examples include "PLAIN", "SCRAM-SHA-256", or "SCRAM-SHA-512".
	SASLMechanism string `yaml:"sasl_mechanism" json:"sasl_mechanism"`

	// ClientID is the OAuth2 application identifier (oauth2 auth).
	ClientID string `yaml:"client_id" json:"client_id"`

	// ClientSecret is the client credential token associated with ClientID (oauth2 auth).
	ClientSecret string `yaml:"client_secret" json:"client_secret"`

	// TokenURL specifies the auth endpoint to fetch OAuth access tokens from (oauth2 auth).
	TokenURL string `yaml:"token_url" json:"token_url"`

	// Scopes list permissions requested under the dynamic OAuth grant (oauth2 auth).
	Scopes []string `yaml:"scopes" json:"scopes"`

	// IAMRoleARN is the Target AWS Role identifier to assume (iam_role auth).
	IAMRoleARN string `yaml:"iam_role_arn" json:"iam_role_arn"`

	// IAMSessionName is a unique tag helping log and isolate the assumed identity session (iam_role auth).
	IAMSessionName string `yaml:"iam_session_name" json:"iam_session_name"`

	// IAMExternalID is the cryptographic passphrase preventing "confused deputy" exploits (iam_role auth).
	IAMExternalID string `yaml:"iam_external_id" json:"iam_external_id"`

	// Region determines the AWS datacentre endpoint to address during session resolution (iam_role auth).
	Region string `yaml:"region" json:"region"`
}

// TLSConfig defines parameters to initialize secure Transport Layer Security connections
// across sources or destinations, accommodating corporate certificate authorities and Mutual TLS.
type TLSConfig struct {
	// CACert contains custom Root CA certificate content or absolute file path.
	CACert string `yaml:"ca_cert" json:"ca_cert"`

	// ClientCert contains the public certificate content or path required during Mutual TLS validation.
	ClientCert string `yaml:"client_cert" json:"client_cert"`

	// ClientKey holds the matching private key required during Mutual TLS validation.
	ClientKey string `yaml:"client_key" json:"client_key"`

	// SkipVerify disables server certificate integrity checks. Useful for offline development environments.
	SkipVerify bool `yaml:"skip_verify" json:"skip_verify"`
}

// PipelineConfig defines the complete YAML configuration schema for a single Auvis ingestion pipeline.
type PipelineConfig struct {
	// Name uniquely identifies the pipeline instance.
	Name string `yaml:"name" json:"name"`

	// Source defines connection, routing, and authentication parameters for the data source.
	Source ConnectorConfig `yaml:"source" json:"source"`

	// Destination defines connection, routing, and authentication parameters for the target data sink.
	Destination ConnectorConfig `yaml:"destination" json:"destination"`

	// Policies configures policy file retrieval and registry settings.
	Policies PoliciesConfig `yaml:"policies" json:"policies"`

	// ErrorHandling establishes the system behavior on validation or evaluation failures.
	ErrorHandling ErrorHandlingConfig `yaml:"error_handling" json:"error_handling"`
}

// ConnectorConfig maps configuration details for a source or destination interface.
type ConnectorConfig struct {
	// Type represents the connector implementation driver name (e.g. "kafka", "confluence", "vector-db").
	Type string `yaml:"type" json:"type"`

	// Settings holds arbitrary key-value options required by the connector driver (e.g. topic names, URLs).
	Settings map[string]string `yaml:"settings" json:"settings"`

	// Auth configures credentials verification for the connection.
	Auth AuthConfig `yaml:"auth" json:"auth"`

	// TLS configures dynamic Transport Layer Security properties.
	TLS TLSConfig `yaml:"tls" json:"tls"`
}

// PoliciesConfig maps the policy rules lookup parameters.
type PoliciesConfig struct {
	// Dir specifies the file path directory holding YAML policy rules files.
	Dir string `yaml:"dir" json:"dir"`
}

// ErrorHandlingConfig maps parameters governing execution loop errors.
type ErrorHandlingConfig struct {
	// FailClosed, if true, suspends record routing to destinations on processing failure.
	FailClosed bool `yaml:"fail_closed" json:"fail_closed"`
}

