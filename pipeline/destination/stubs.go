package destination

import (
	"errors"

	"github.com/Auvis-Labs/auvis/pipeline"
	pipelinecfg "github.com/Auvis-Labs/auvis/pipeline/config"
)

var errEnterpriseOnly = errors.New("this destination connector requires Auvis Enterprise")

// StubDestination is a placeholder for connectors available in Auvis Enterprise.
type StubDestination struct {
	name string
}

func (s *StubDestination) Connect() error {
	return errEnterpriseOnly
}

func (s *StubDestination) WriteRecord(record pipeline.Record) error {
	return errEnterpriseOnly
}

func (s *StubDestination) Flush() error {
	return nil
}

func (s *StubDestination) Close() error {
	return nil
}

type PgVectorDestination struct{ StubDestination }
type PineconeDestination struct{ StubDestination }
type PostgresDestination struct{ StubDestination }
type WeaviateDestination struct{ StubDestination }

func NewPgVectorDestination(settings map[string]string, auth pipelinecfg.AuthConfig, tls pipelinecfg.TLSConfig) *PgVectorDestination {
	return &PgVectorDestination{StubDestination{name: "pgvector"}}
}

func NewPineconeDestination(settings map[string]string, auth pipelinecfg.AuthConfig, tlsCfg pipelinecfg.TLSConfig) *PineconeDestination {
	return &PineconeDestination{StubDestination{name: "pinecone"}}
}

func NewPostgresDestination(settings map[string]string, auth pipelinecfg.AuthConfig, tls pipelinecfg.TLSConfig) *PostgresDestination {
	return &PostgresDestination{StubDestination{name: "postgres"}}
}

func NewWeaviateDestination(settings map[string]string, auth pipelinecfg.AuthConfig, tlsCfg pipelinecfg.TLSConfig) *WeaviateDestination {
	return &WeaviateDestination{StubDestination{name: "weaviate"}}
}

