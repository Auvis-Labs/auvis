package source

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cridiv/Auvis/pipeline"
	config "github.com/cridiv/Auvis/pipeline/config"
	"github.com/cridiv/Auvis/pipeline/decoder"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// KafkaSource implements pipeline.Source to read records from one or more Kafka topics.
type KafkaSource struct {
	brokers []string
	topics  []string
	groupID string
	auth    config.AuthConfig
	tls     config.TLSConfig

	// settings stores general key-value configs (like auto_offset_reset)
	settings map[string]string

	readers     []*kafka.Reader
	dialer      *kafka.Dialer
	mu          sync.Mutex
	srClient    *decoder.SchemaRegistryClient
	avroDec     decoder.Decoder
	protoDec    decoder.Decoder
	dlqWriter   *DLQWriter
	exactlyOnce bool
	bgCancel    context.CancelFunc
	metrics     pipeline.MetricsReporter
}

// NewKafkaSource creates a new configured KafkaSource instance.
func NewKafkaSource(settings map[string]string, auth config.AuthConfig, tls config.TLSConfig) *KafkaSource {
	brokersStr := settings["brokers"]
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	// Parse topics: support comma-separated "topics" key, fall back to singular "topic"
	var topics []string
	if topicsStr, ok := settings["topics"]; ok && topicsStr != "" {
		for _, t := range strings.Split(topicsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topics = append(topics, t)
			}
		}
	} else if singleTopic := settings["topic"]; singleTopic != "" {
		topics = []string{singleTopic}
	}

	k := &KafkaSource{
		brokers:     brokers,
		topics:      topics,
		groupID:     settings["group_id"],
		auth:        auth,
		tls:         tls,
		settings:    settings,
		exactlyOnce: strings.ToLower(settings["delivery_semantics"]) == "exactly_once",
		metrics:     &pipeline.NoOpReporter{},
	}

	if srURL, ok := settings["schema_registry_url"]; ok && srURL != "" {
		srUser := settings["schema_registry_user"]
		srPass := settings["schema_registry_password"]
		srSkipVerify := strings.ToLower(settings["schema_registry_skip_verify"]) == "true"
		k.srClient = decoder.NewSchemaRegistryClient(srURL, srUser, srPass, srSkipVerify)
		k.avroDec = decoder.NewAvroDecoder()
		k.protoDec = decoder.NewProtobufDecoder()
	}

	return k
}

// Connect sets up SASL, TLS, and starts the reader connection(s).
func (k *KafkaSource) Connect() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(k.readers) > 0 {
		return nil
	}

	if len(k.brokers) == 0 || k.brokers[0] == "" {
		return errors.New("kafka brokers must be specified")
	}
	if len(k.topics) == 0 {
		return errors.New("kafka topic must be specified")
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	// 1. Setup SASL Authentication
	if k.auth.Type == config.AuthTypeSASLSSL {
		mech, err := getSASLMechanism(k.auth)
		if err != nil {
			return fmt.Errorf("failed to setup SASL mechanism: %w", err)
		}
		dialer.SASLMechanism = mech
	}

	// 2. Setup TLS Secure transport
	hasTLS := k.tls.CACert != "" || k.tls.ClientCert != "" || k.tls.SkipVerify
	if k.auth.Type == config.AuthTypeSASLSSL || hasTLS {
		tlsConfig, err := getTLSConfig(k.tls)
		if err != nil {
			return fmt.Errorf("failed to configure TLS: %w", err)
		}
		dialer.TLS = tlsConfig
	}

	k.dialer = dialer

	// 3. Resolve Offset Reset strategy
	var startOffset int64 = kafka.LastOffset
	if strings.ToLower(k.settings["auto_offset_reset"]) == "earliest" {
		startOffset = kafka.FirstOffset
	}

	// 4. Build reader(s)
	if k.groupID != "" {
		// Consumer group mode: single reader handles multiple topics via GroupTopics
		rcfg := kafka.ReaderConfig{
			Brokers:     k.brokers,
			Dialer:      dialer,
			StartOffset: startOffset,
			GroupID:     k.groupID,
		}
		if len(k.topics) == 1 {
			rcfg.Topic = k.topics[0]
		} else {
			rcfg.GroupTopics = k.topics
		}
		if k.exactlyOnce {
			// Disable auto-commit; offsets are committed explicitly via AckFunc
			rcfg.CommitInterval = 0
		} else {
			k.applyCommitInterval(&rcfg)
		}
		k.readers = []*kafka.Reader{kafka.NewReader(rcfg)}
	} else {
		// No consumer group: spawn one reader per topic
		for _, topic := range k.topics {
			rcfg := kafka.ReaderConfig{
				Brokers:     k.brokers,
				Topic:       topic,
				Dialer:      dialer,
				StartOffset: startOffset,
			}
			if k.exactlyOnce {
				rcfg.CommitInterval = 0
			} else {
				k.applyCommitInterval(&rcfg)
			}
			k.readers = append(k.readers, kafka.NewReader(rcfg))
		}
	}

	// 5. Initialize DLQ writer if configured
	if dlqTopic, ok := k.settings["dead_letter_topic"]; ok && dlqTopic != "" {
		k.dlqWriter = NewDLQWriter(k.brokers, dlqTopic, dialer)
		slog.Info("Dead-letter queue writer initialized", "dlq_topic", dlqTopic)
	}

	// 6. Resolve Metrics provider
	if strings.ToLower(k.settings["metrics_provider"]) == "prometheus" {
		port := k.settings["metrics_port"]
		if port == "" {
			port = "9090" // Default port
		}
		k.metrics = NewPrometheusReporter(port)
	}

	// 7. Start background metrics polling goroutine if needed
	bgCtx, bgCancel := context.WithCancel(context.Background())
	k.bgCancel = bgCancel
	for _, reader := range k.readers {
		go func(r *kafka.Reader) {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-bgCtx.Done():
					return
				case <-ticker.C:
					stats := r.Stats()
					topic := stats.Topic
					if topic == "" {
						topic = "unknown"
					}
					partition := stats.Partition
					if partition == "" {
						partition = "all"
					}
					k.metrics.ReportLag(topic, partition, k.groupID, stats.Lag)
				}
			}
		}(reader)
	}

	slog.Info("Kafka source connector connected successfully", "brokers", k.brokers, "topics", k.topics, "group_id", k.groupID)
	return nil
}

// applyCommitInterval parses the commit_interval setting and applies it to a ReaderConfig.
func (k *KafkaSource) applyCommitInterval(cfg *kafka.ReaderConfig) {
	if intervalStr, ok := k.settings["commit_interval"]; ok && intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			cfg.CommitInterval = d
		} else {
			slog.Warn("Invalid commit_interval format, using default", "provided", intervalStr)
		}
	}
}

// ReadRecords streams records from all configured Kafka topics into the pipeline.
func (k *KafkaSource) ReadRecords(ctx context.Context) <-chan pipeline.Record {
	ch := make(chan pipeline.Record, 100)

	var wg sync.WaitGroup
	for _, reader := range k.readers {
		wg.Add(1)
		go func(r *kafka.Reader) {
			defer wg.Done()
			k.readFromReader(ctx, r, ch)
		}(reader)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}

// readFromReader reads messages from a single kafka.Reader and sends decoded records to the channel.
func (k *KafkaSource) readFromReader(ctx context.Context, reader *kafka.Reader, ch chan<- pipeline.Record) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var msg kafka.Message
		var err error

		if k.exactlyOnce {
			// Fetch without committing — offset is committed via AckFunc after processing
			msg, err = reader.FetchMessage(ctx)
		} else {
			// Read and auto-commit (at-least-once)
			msg, err = reader.ReadMessage(ctx)
		}

		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, ioEOF(err)) {
				return
			}
			slog.Error("Kafka read error, skipping message", "error", err)
			continue
		}

		// Update consumer lag metric
		stats := reader.Stats()
		topic := stats.Topic
		if topic == "" {
			topic = "unknown"
		}
		partition := stats.Partition
		if partition == "" {
			partition = "all"
		}
		k.metrics.ReportLag(topic, partition, k.groupID, stats.Lag)

		// Map message headers
		headers := make(map[string]string)
		for _, h := range msg.Headers {
			headers[h.Key] = string(h.Value)
		}

		// Parse payload dynamically
		var data map[string]any
		var payload []byte = msg.Value
		var decodeErr error
		decoded := false

		if k.srClient != nil {
			schemaID := -1

			// 1. Check explicitly in headers (e.g. "schema_id")
			if idStr, ok := headers["schema_id"]; ok {
				if id, err := strconv.Atoi(idStr); err == nil {
					schemaID = id
				}
			}

			// 2. Fallback to Confluent Wire Format check
			if schemaID == -1 && len(payload) > 5 && payload[0] == 0x00 {
				schemaID = int(binary.BigEndian.Uint32(payload[1:5]))
				payload = payload[5:] // strip wire format prefix
			}

			if schemaID != -1 {
				schema, err := k.srClient.GetSchema(schemaID)
				if err == nil && schema != nil {
					sTyp := schema.SchemaType()
					if sTyp == nil || *sTyp == "AVRO" {
						data, decodeErr = k.avroDec.Decode(schema.Schema(), payload)
						if decodeErr == nil {
							decoded = true
						}
					} else if *sTyp == "PROTOBUF" {
						data, decodeErr = k.protoDec.Decode(schema.Schema(), payload)
						if decodeErr == nil {
							decoded = true
						}
					}
				} else {
					decodeErr = err
				}
			}
		}

		if !decoded {
			if err := json.Unmarshal(payload, &data); err != nil {
				slog.Warn("Failed to decode message payload, falling back to raw data wrapper", "error", err, "sr_error", decodeErr)

				errStr := err.Error()
				if decodeErr != nil {
					errStr = fmt.Sprintf("json_err: %v | sr_err: %v", err, decodeErr)
				}

				data = map[string]any{
					"_raw_bytes": payload,
					"_raw_error": errStr,
				}
			}
		}

		// Populate message transport metadata
		metadata := map[string]any{
			"kafka.topic":     msg.Topic,
			"kafka.partition": msg.Partition,
			"kafka.offset":    msg.Offset,
			"kafka.key":       string(msg.Key),
			"kafka.headers":   headers,
		}

		record := pipeline.Record{
			ID:        fmt.Sprintf("%s-%d-%d", msg.Topic, msg.Partition, msg.Offset),
			Source:    fmt.Sprintf("kafka.%s", msg.Topic),
			Data:      data,
			Metadata:  metadata,
			Timestamp: msg.Time,
		}

		if k.exactlyOnce {
			r := reader
			m := msg
			record.AckFunc = func() error {
				commitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return r.CommitMessages(commitCtx, m)
			}
		}

		select {
		case <-ctx.Done():
			return
		case ch <- record:
		}
	}
}

// Close closes all underlying Kafka readers and the DLQ writer.
func (k *KafkaSource) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.bgCancel != nil {
		k.bgCancel()
	}

	var firstErr error
	if k.metrics != nil {
		if err := k.metrics.Close(); err != nil {
			firstErr = err
		}
	}

	for _, r := range k.readers {
		if err := r.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	k.readers = nil

	if k.dlqWriter != nil {
		if err := k.dlqWriter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		k.dlqWriter = nil
	}

	slog.Info("Kafka source connector closed gracefully")
	return firstErr
}

// DLQ returns the dead-letter queue writer, or nil if not configured.
func (k *KafkaSource) DLQ() *DLQWriter {
	return k.dlqWriter
}

func ioEOF(err error) error {
	if err != nil && err.Error() == "EOF" {
		return err
	}
	return nil
}

func getSASLMechanism(auth config.AuthConfig) (sasl.Mechanism, error) {
	switch strings.ToUpper(auth.SASLMechanism) {
	case "PLAIN":
		return plain.Mechanism{
			Username: auth.Username,
			Password: auth.Password,
		}, nil
	case "SCRAM-SHA-256":
		m, err := scram.Mechanism(scram.SHA256, auth.Username, auth.Password)
		if err != nil {
			return nil, err
		}
		return m, nil
	case "SCRAM-SHA-512":
		m, err := scram.Mechanism(scram.SHA512, auth.Username, auth.Password)
		if err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s", auth.SASLMechanism)
	}
}

func getTLSConfig(t config.TLSConfig) (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: t.SkipVerify,
	}

	if t.CACert != "" {
		pool := x509.NewCertPool()
		var ok bool
		if strings.Contains(t.CACert, "-----BEGIN CERTIFICATE-----") {
			ok = pool.AppendCertsFromPEM([]byte(t.CACert))
		} else {
			data, err := os.ReadFile(t.CACert)
			if err != nil {
				return nil, err
			}
			ok = pool.AppendCertsFromPEM(data)
		}
		if !ok {
			return nil, errors.New("failed to parse CA certificate")
		}
		config.RootCAs = pool
	}

	if t.ClientCert != "" && t.ClientKey != "" {
		var cert tls.Certificate
		var err error
		if strings.Contains(t.ClientCert, "-----BEGIN CERTIFICATE-----") {
			cert, err = tls.X509KeyPair([]byte(t.ClientCert), []byte(t.ClientKey))
		} else {
			cert, err = tls.LoadX509KeyPair(t.ClientCert, t.ClientKey)
		}
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}
