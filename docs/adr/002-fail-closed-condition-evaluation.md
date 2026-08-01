# ADR 002: Fail-Closed Security Policy for Indeterminate Condition Evaluation

- **Status:** Accepted
- **Date:** July 28, 2026

## Context

Auvis supports sandboxed CEL condition expressions on policy rules (e.g. `condition: "user.clearance != 'high'"` or `filter_condition: "record.status == 'active'"`). During runtime execution, an incoming HTTP request or pipeline record might lack specific context attributes referenced by the expression (e.g. missing `user.clearance` header or null record field).

If an expression cannot be evaluated due to missing attributes or evaluation errors, the system encounters an **indeterminate state**.

## Decision

Auvis strictly enforces a **Fail-Closed Security Strategy** across all condition evaluations:

1. **Field-Level Rule Conditions:** If a rule condition is indeterminate, Auvis treats the condition as **true** (the rule is enforced, masking/removing the target field).
2. **Row-Level Array Filtering (`filter_condition`):** If a row-level filter condition is indeterminate, Auvis **excludes the row** from the output array.
3. **Audit Logging:** Every fail-closed fallback event logs an explicit warning (`slog.Warn("Filter condition attribute missing — fail-closed row exclusion applied")`) containing the Request ID.

## Consequences

- **Security Guarantee:** Guarantees zero data leakage of sensitive PII or unauthorized rows when request context attributes are omitted or malformed.
- **Operator Visibility:** Warnings highlight missing headers or missing attributes in audit logs without exposing raw PII.
