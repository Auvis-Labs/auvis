# Security Policy

Auvis is a data governance proxy that sits in the path of sensitive data — PII, PHI, financial records — before it reaches an AI system. Given what it protects, security issues in Auvis are treated as a priority, not a backlog item.

We appreciate the security research community's help in keeping Auvis and its users safe. This document explains how to report a vulnerability responsibly.

---

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release (main branch) | ✅ |
| Older tagged releases | ❌ (please upgrade before reporting) |

Auvis is under active development. Only the latest release receives security fixes at this stage.

---

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.** Public issues are visible to everyone, including anyone who might exploit the report before a fix ships.

### How to report

**Preferred: GitHub Private Vulnerability Reporting**
Use the "Report a vulnerability" button under this repository's Security tab. This creates a private advisory visible only to maintainers until a fix is ready.

**Alternative: Email**  
Send a report to **`ademoladeremi0@gmail.com`**.

Please include, where possible:

- A clear description of the vulnerability and its potential impact
- Steps to reproduce, or a proof-of-concept if available
- The version or commit hash affected
- Whether the issue affects the policy engine, transform engine, PII scanner, audit logger, or another component
- Any suggested remediation, if you have one (not required)

### What to expect

| Stage | Timeline |
|-------|----------|
| Acknowledgment of your report | Within 3 business days |
| Initial assessment and severity triage | Within 7 business days |
| Fix development, for confirmed vulnerabilities | Timeline communicated after triage, prioritized by severity |
| Public disclosure | Coordinated with the reporter, typically after a fix is released |

We will keep you updated as we work through your report. We ask for reasonable time to investigate and patch before any public disclosure, and we're happy to coordinate a disclosure timeline that works for you.

---

## Scope

### In scope

- The Auvis proxy/gateway itself (policy engine, transform engine, PII scanner, audit logger, destination connectors)
- Authentication and context resolution (JWT handling, header parsing, IdP lookup)
- The policy DSL parser and condition evaluator
- Any bypass of masking, redaction, or policy enforcement
- Any way the audit trail could be made inaccurate, incomplete, or tampered with undetected
- Denial-of-service vectors specific to Auvis's request handling

### Out of scope

- Vulnerabilities in third-party dependencies (please report these upstream — though we'd appreciate a heads-up so we can track and patch our usage)
- Issues requiring physical access to a deployment
- Social engineering
- Vulnerabilities in a customer's own upstream systems, unrelated to Auvis itself
- Missing security best practices that don't constitute an exploitable vulnerability (e.g. suggestions are welcome as a regular issue or discussion, not a security report)

---

## Our Commitment

We will not take legal action against researchers who:

- Make a good-faith effort to avoid privacy violations, data destruction, or service disruption during their research
- Report vulnerabilities privately and give us reasonable time to address them before any public disclosure
- Do not access, modify, or exfiltrate data beyond what's necessary to demonstrate the vulnerability

We're an early-stage, largely solo-maintained project. We take security seriously despite that, but please be patient with response times — we're committed to the timelines above and will do our best to beat them.

---

## A Note on Transparency

Auvis is open source specifically because we believe a data governance tool should be verifiable, not a black box you're asked to trust blindly. That same principle applies to how we handle security issues: once a vulnerability is fixed and responsibly disclosed, we document it in the [CHANGELOG](./CHANGELOG.md) and, for anything significant, in a public security advisory — so users always know what was fixed and why.

Thank you for helping keep Auvis, and the sensitive data it protects, secure.