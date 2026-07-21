package pipeline

import (
	"os"
	"testing"
)

func TestResolveEnvVars(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("TEST_KAFKA_USER", "admin")
	os.Setenv("TEST_KAFKA_SECRET", "super-secret-key-123")
	defer func() {
		os.Unsetenv("TEST_KAFKA_USER")
		os.Unsetenv("TEST_KAFKA_SECRET")
	}()

	inputYAML := `
source:
  type: kafka
  auth:
    username: ${TEST_KAFKA_USER}
    password: ${TEST_KAFKA_SECRET}
    client_secret: ${NON_EXISTENT_VAR}
  custom_regex: "$\\{not-matching-unescaped"
`

	expectedYAML := `
source:
  type: kafka
  auth:
    username: admin
    password: super-secret-key-123
    client_secret: 
  custom_regex: "$\\{not-matching-unescaped"
`

	resolved := ResolveEnvVars([]byte(inputYAML))

	if string(resolved) != expectedYAML {
		t.Errorf("expected expanded YAML to be:\n%s\n\nbut got:\n%s", expectedYAML, string(resolved))
	}
}

func TestParsePipelineConfig(t *testing.T) {
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	os.Setenv("KAFKA_PASS", "kpass")
	defer func() {
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("KAFKA_PASS")
	}()

	yamlData := `
name: "customer-ingest-pipeline"
source:
  type: "kafka"
  settings:
    brokers: "${KAFKA_BROKERS}"
    topic: "customer-events"
    group_id: "auvis-consumer"
  auth:
    type: "sasl_ssl"
    username: "admin"
    password: "${KAFKA_PASS}"
    sasl_mechanism: "PLAIN"
  tls:
    skip_verify: true
destination:
  type: "vector-db"
  settings:
    url: "http://qdrant:6333"
    collection: "customer_embeddings"
  auth:
    type: "api_key"
    api_key: "secret-qdrant-token"
policies:
  dir: "./policies/pipelines"
error_handling:
  fail_closed: true
`

	cfg, err := ParsePipelineConfig([]byte(yamlData))
	if err != nil {
		t.Fatalf("expected no error parsing config, got %v", err)
	}

	if cfg.Name != "customer-ingest-pipeline" {
		t.Errorf("expected Name to be 'customer-ingest-pipeline', got %s", cfg.Name)
	}

	if cfg.Source.Type != "kafka" {
		t.Errorf("expected source.type to be 'kafka', got %s", cfg.Source.Type)
	}

	if brokers := cfg.Source.Settings["brokers"]; brokers != "localhost:9092" {
		t.Errorf("expected source brokers setting to resolve to 'localhost:9092', got %s", brokers)
	}

	if cfg.Source.Auth.Type != AuthTypeSASLSSL {
		t.Errorf("expected source auth type to be 'sasl_ssl', got %s", cfg.Source.Auth.Type)
	}

	if cfg.Source.Auth.Password != "kpass" {
		t.Errorf("expected source password to resolve to 'kpass', got %s", cfg.Source.Auth.Password)
	}

	if !cfg.Source.TLS.SkipVerify {
		t.Error("expected source tls skip_verify to be true")
	}

	if cfg.Destination.Type != "vector-db" {
		t.Errorf("expected destination type to be 'vector-db', got %s", cfg.Destination.Type)
	}

	if cfg.Destination.Auth.APIKey != "secret-qdrant-token" {
		t.Errorf("expected destination api_key to be 'secret-qdrant-token', got %s", cfg.Destination.Auth.APIKey)
	}

	if cfg.Policies.Dir != "./policies/pipelines" {
		t.Errorf("expected policies dir to be './policies/pipelines', got %s", cfg.Policies.Dir)
	}

	if !cfg.ErrorHandling.FailClosed {
		t.Error("expected error_handling.fail_closed to be true")
	}
}

