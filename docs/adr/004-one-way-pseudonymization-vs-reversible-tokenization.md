# ADR 004: One-Way Deterministic Pseudonymization vs Reversible Tokenization

- **Status:** Accepted
- **Date:** July 30, 2026

## Context

AI context tracking and vector index querying require consistent entity representation (e.g. Patient `PT-101` or Email `alice@example.com` must always produce the same surrogate value across multiple LLM prompts). Traditional tokenization vaults maintain central database tables linking tokens back to raw PII. If a token vault is compromised, all sensitive PII is exposed.

## Decision

Auvis implements **One-Way Deterministic Format-Preserving Pseudonymization (`action: pseudonymize`)** using keyed `HMAC-SHA256(seed, value)` without a reversible vault:

1. **Vaultless Architecture:** No database or vault stores mappings between original PII and pseudonyms. The transformation is mathematically non-reversible.
2. **Determinism:** The same input value always produces the exact same pseudonymous output (`amara@gmail.com` $\rightarrow$ `usr_3a9f81b2c4@gmail.com`), allowing vector database index querying and cross-prompt correlation.
3. **Format Preservation:** Email domains, SSN formatting, and ID prefixes are preserved to maintain downstream application parsing compatibility.
4. **Per-Rule Seed Isolation:** Supports optional per-rule `seed` overrides (`seed: "dept-patients"`) to prevent cross-tenant correlation attacks.

## Consequences

- **Breach Immunity:** Even if an attacker gains access to logs or vector databases, pseudonyms cannot be reversed to raw PII.
- **Zero Database Overhead:** Executes in $<0.001\text{ ms}$ in-memory without database roundtrips.
