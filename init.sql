-- ─────────────────────────────────────────
-- Plain Postgres destination table
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS governed_records (
    id          TEXT PRIMARY KEY,
    source      TEXT,
    data        JSONB NOT NULL,
    metadata    JSONB,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_governed_records_source ON governed_records (source);
CREATE INDEX IF NOT EXISTS idx_governed_records_created_at ON governed_records (created_at);

-- ─────────────────────────────────────────
-- pgvector destination table
-- ─────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS governed_vectors (
    id          TEXT PRIMARY KEY,
    source      TEXT,
    embedding   vector(1536),   -- adjust dimension to match your embedding model
    metadata    JSONB,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_governed_vectors_embedding
    ON governed_vectors USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);