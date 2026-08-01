package source

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cridiv/Auvis/pipeline"

	"github.com/segmentio/kafka-go"
)

// DLQWriter produces failed records to a Kafka dead-letter topic.
type DLQWriter struct {
	writer *kafka.Writer
	mu     sync.Mutex
}

// NewDLQWriter creates a DLQ producer targeting the specified topic.
func NewDLQWriter(brokers []string, topic string, dialer *kafka.Dialer) *DLQWriter {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	if dialer != nil {
		w.Transport = &kafka.Transport{
			TLS:  dialer.TLS,
			SASL: dialer.SASLMechanism,
		}
	}
	return &DLQWriter{writer: w}
}

// Write serializes a failed record and produces it to the dead-letter topic.
// The original Kafka transport metadata and the failure reason are attached as headers.
func (d *DLQWriter) Write(ctx context.Context, record pipeline.Record, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := json.Marshal(record.Data)
	if err != nil {
		return fmt.Errorf("dlq: failed to marshal record data: %w", err)
	}

	headers := []kafka.Header{
		{Key: "dlq.reason", Value: []byte(reason)},
		{Key: "dlq.original_id", Value: []byte(record.ID)},
		{Key: "dlq.original_source", Value: []byte(record.Source)},
	}

	// Carry over original Kafka metadata if available
	if record.Metadata != nil {
		if topic, ok := record.Metadata["kafka.topic"].(string); ok {
			headers = append(headers, kafka.Header{Key: "dlq.original_topic", Value: []byte(topic)})
		}
		if partition, ok := record.Metadata["kafka.partition"].(int); ok {
			headers = append(headers, kafka.Header{Key: "dlq.original_partition", Value: []byte(fmt.Sprintf("%d", partition))})
		}
		if offset, ok := record.Metadata["kafka.offset"].(int64); ok {
			headers = append(headers, kafka.Header{Key: "dlq.original_offset", Value: []byte(fmt.Sprintf("%d", offset))})
		}
	}

	var key []byte
	if record.Metadata != nil {
		if k, ok := record.Metadata["kafka.key"].(string); ok && k != "" {
			key = []byte(k)
		}
	}

	msg := kafka.Message{
		Key:     key,
		Value:   data,
		Headers: headers,
	}

	if err := d.writer.WriteMessages(ctx, msg); err != nil {
		slog.Error("DLQ write failed", "record_id", record.ID, "error", err)
		return fmt.Errorf("dlq: failed to produce message: %w", err)
	}

	slog.Info("Record routed to dead-letter topic", "record_id", record.ID, "reason", reason)
	return nil
}

// Close flushes and closes the DLQ writer.
func (d *DLQWriter) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.writer == nil {
		return nil
	}
	err := d.writer.Close()
	d.writer = nil
	return err
}
