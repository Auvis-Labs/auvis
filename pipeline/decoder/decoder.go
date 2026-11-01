package decoder

import (
	"github.com/riferrei/srclient"
)

// Decoder defines the interface for dynamically deserializing incoming binary messages.
type Decoder interface {
	Decode(schema string, payload []byte) (map[string]any, error)
}

// SchemaRegistryClient manages fetching schema definitions for serialised payloads.
type SchemaRegistryClient struct{}

func NewSchemaRegistryClient(url, user, pass string, skipVerify bool) *SchemaRegistryClient {
	return &SchemaRegistryClient{}
}

func (c *SchemaRegistryClient) GetSchema(id int) (*srclient.Schema, error) {
	return nil, nil
}

// StubDecoder provides a fallback no-op decoder for community editions.
type StubDecoder struct{}

func (s *StubDecoder) Decode(schema string, payload []byte) (map[string]any, error) {
	return nil, nil
}

func NewAvroDecoder() Decoder {
	return &StubDecoder{}
}

func NewProtobufDecoder() Decoder {
	return &StubDecoder{}
}

