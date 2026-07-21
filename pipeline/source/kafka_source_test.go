package source

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	config "github.com/Auvis-Labs/auvis/pipeline/config"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewKafkaSource(t *testing.T) {
	settings := map[string]string{
		"brokers":           "127.0.0.1:9092, 127.0.0.1:9093",
		"topic":             "test-topic",
		"group_id":          "test-group",
		"auto_offset_reset": "earliest",
	}

	auth := config.AuthConfig{
		Type:          config.AuthTypeSASLSSL,
		Username:      "user",
		Password:      "pass",
		SASLMechanism: "plain",
	}

	tlsConfig := config.TLSConfig{
		SkipVerify: true,
	}

	source := NewKafkaSource(settings, auth, tlsConfig)

	if len(source.brokers) != 2 {
		t.Fatalf("expected 2 brokers, got %d", len(source.brokers))
	}
	if source.brokers[0] != "127.0.0.1:9092" || source.brokers[1] != "127.0.0.1:9093" {
		t.Errorf("incorrect brokers mapping, got %v", source.brokers)
	}
	if len(source.topics) != 1 || source.topics[0] != "test-topic" {
		t.Errorf("expected topics ['test-topic'], got %v", source.topics)
	}
	if source.groupID != "test-group" {
		t.Errorf("expected groupID 'test-group', got '%s'", source.groupID)
	}
	if source.auth.Username != "user" || source.auth.Password != "pass" {
		t.Errorf("incorrect auth propagation")
	}
	if !source.tls.SkipVerify {
		t.Errorf("expected SkipVerify true")
	}
}

func TestNewKafkaSource_MultiTopic(t *testing.T) {
	// Multi-topic via "topics" key
	settings := map[string]string{
		"brokers":  "localhost:9092",
		"topics":   "events, logs, metrics",
		"group_id": "multi-group",
	}
	src := NewKafkaSource(settings, config.AuthConfig{}, config.TLSConfig{})
	if len(src.topics) != 3 {
		t.Fatalf("expected 3 topics, got %d: %v", len(src.topics), src.topics)
	}
	if src.topics[0] != "events" || src.topics[1] != "logs" || src.topics[2] != "metrics" {
		t.Errorf("incorrect topics parsing, got %v", src.topics)
	}

	// "topics" takes precedence over "topic"
	settings2 := map[string]string{
		"brokers": "localhost:9092",
		"topic":   "should-be-ignored",
		"topics":  "a,b",
	}
	src2 := NewKafkaSource(settings2, config.AuthConfig{}, config.TLSConfig{})
	if len(src2.topics) != 2 || src2.topics[0] != "a" || src2.topics[1] != "b" {
		t.Errorf("topics should take precedence over topic, got %v", src2.topics)
	}
}

func TestKafkaSource_Connect_Validation(t *testing.T) {
	// Missing Brokers
	s1 := NewKafkaSource(map[string]string{"topic": "t"}, config.AuthConfig{}, config.TLSConfig{})
	if err := s1.Connect(); err == nil || err.Error() != "kafka brokers must be specified" {
		t.Errorf("expected broker validation error, got %v", err)
	}

	// Missing Topic
	s2 := NewKafkaSource(map[string]string{"brokers": "b"}, config.AuthConfig{}, config.TLSConfig{})
	if err := s2.Connect(); err == nil || err.Error() != "kafka topic must be specified" {
		t.Errorf("expected topic validation error, got %v", err)
	}
}

func TestGetSASLMechanism(t *testing.T) {
	// Test PLAIN mechanism
	auth := config.AuthConfig{
		Type:          config.AuthTypeSASLSSL,
		Username:      "alice",
		Password:      "secret",
		SASLMechanism: "PLAIN",
	}
	mech, err := getSASLMechanism(auth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech.Name() != "PLAIN" {
		t.Errorf("expected mechanism PLAIN, got %s", mech.Name())
	}

	// Test SCRAM-SHA-256
	auth.SASLMechanism = "SCRAM-SHA-256"
	mech, err = getSASLMechanism(auth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech.Name() != "SCRAM-SHA-256" {
		t.Errorf("expected mechanism SCRAM-SHA-256, got %s", mech.Name())
	}

	// Test unsupported
	auth.SASLMechanism = "UNSUPPORTED"
	_, err = getSASLMechanism(auth)
	if err == nil {
		t.Error("expected error for unsupported mechanism, got nil")
	}
}

func TestGetTLSConfig(t *testing.T) {
	// Generate valid test certificate and private key
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("failed to generate test cert: %v", err)
	}

	// Test SkipVerify
	tc := config.TLSConfig{SkipVerify: true}
	cfg, err := getTLSConfig(tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}

	// Test invalid CA PEM
	tc = config.TLSConfig{CACert: "invalid-pem"}
	_, err = getTLSConfig(tc)
	if err == nil {
		t.Error("expected error for invalid CA PEM, got nil")
	}

	// Test valid CA PEM inline
	tc = config.TLSConfig{CACert: string(certPEM)}
	cfg, err = getTLSConfig(tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Error("expected RootCAs pool to be loaded")
	}

	// Test invalid mTLS Cert inline
	tc = config.TLSConfig{
		ClientCert: "invalid-cert",
		ClientKey:  "invalid-key",
	}
	_, err = getTLSConfig(tc)
	if err == nil {
		t.Error("expected error for invalid cert key pairs, got nil")
	}

	// Test valid mTLS inline keys
	tc = config.TLSConfig{
		ClientCert: string(certPEM),
		ClientKey:  string(keyPEM),
	}
	cfg, err = getTLSConfig(tc)
	if err != nil {
		t.Fatalf("unexpected error loading valid mTLS keypair: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Error("expected Certificates to be loaded")
	}
}

func generateTestCert() (certPEM []byte, keyPEM []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}
	certPEM = pem.EncodeToMemory(certBlock)

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}
	keyPEM = pem.EncodeToMemory(keyBlock)

	return certPEM, keyPEM, nil
}

func TestKafkaSource_Lifecycle(t *testing.T) {
	// Basic lifecycle check
	settings := map[string]string{
		"brokers": "localhost:9092",
		"topic":   "test-topic",
	}
	src := NewKafkaSource(settings, config.AuthConfig{}, config.TLSConfig{})

	// Closing a non-connected source should be a safe no-op
	if err := src.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestExactlyOnceFlag(t *testing.T) {
	// 1. exactly_once delivery semantics
	settings := map[string]string{
		"brokers":            "localhost:9092",
		"topic":              "test-topic",
		"delivery_semantics": "exactly_once",
	}
	src := NewKafkaSource(settings, config.AuthConfig{}, config.TLSConfig{})
	if !src.exactlyOnce {
		t.Error("expected exactlyOnce to be true when delivery_semantics is exactly_once")
	}

	// 2. default delivery semantics
	settingsDefault := map[string]string{
		"brokers": "localhost:9092",
		"topic":   "test-topic",
	}
	srcDefault := NewKafkaSource(settingsDefault, config.AuthConfig{}, config.TLSConfig{})
	if srcDefault.exactlyOnce {
		t.Error("expected exactlyOnce to be false by default")
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Initialize a PrometheusReporter on an empty port (which bypasses http server start)
	reporter := NewPrometheusReporter("")
	defer reporter.Close()
	
	// Set metrics
	reporter.ReportLag("test-topic", "0", "test-group-id", 42)
	
	// Gather and verify metric value.
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "auvis_kafka_consumer_lag" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find auvis_kafka_consumer_lag metric registered")
	}
}

