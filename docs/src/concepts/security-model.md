---
title: "Security model"
description: "Authentication, the deny-by-default authorization pipeline, tenant isolation, and crypto."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Security model

Capper's security rests on three pillars: **authentication** (who are you),
**authorization** (what may you do), and **tenant isolation** (whose data is it).
Plus the cryptographic primitives that protect tokens and secrets.

## Authentication

- **Bearer tokens.** Principals authenticate with HMAC-SHA256-signed bearer tokens
  (`v1.<payload>.<sig>`). The signing key is a 32-byte `crypto/rand` key stored
  `0600` in the store. Verification is constant-time (`hmac.Equal`), checks expiry,
  and confirms the token still exists (revocable). A short-TTL cache keeps the
  per-request revocation check off the hot path.
- **Sessions.** The Web console exchanges a token for a `Secure`, `HttpOnly`,
  `SameSite=Strict` session cookie plus a double-submit CSRF token. Because cookies
  are `Secure`, browser sessions require HTTPS.
- **Node join tokens** authenticate a node joining the topology (`capper node join`).

## Authorization (deny-by-default)

Every authorizing request builds an `authz.AuthContext` and runs a fixed pipeline.
**The default decision is DENY; an explicit deny always beats any allow:**

1. Resolve the principal from the token.
2. Ensure the org and account are **active** (suspended → denied).
3. Apply hard system denies (e.g. no principal).
4. Evaluate **organization guardrails** — an explicit deny here cannot be
   overridden, even by root.
5. **Org-root / account-root** principals bypass account IAM (but not guardrails);
   `system` principals are allowed for internal calls.
6. Evaluate **account IAM policies** — explicit deny wins; otherwise at least one
   matching allow is required.
7. Verify **resource ownership**: the resource's `account_id` must match the
   caller's account (or an assumed acting account).

Principal types: `org-root-user`, `account-root-user`, `iam-user`, `iam-role`,
`service-account`, `system`. Cross-account access is explicit via role assumption
(`capper iam assume-role`) and account trust.

## Tenant isolation

The org/account/project scope of a request is resolved server-side. The
`X-Capper-Org-ID` / `-Account-ID` / `-Project-ID` headers may **narrow** to a
tenant the principal belongs to, but can never widen scope — they are validated
against the principal's memberships and root status, and an unauthorized scope
request is rejected (403). Resource ownership (step 7 above) is the final isolation
gate.

## The HTTP edge

- **TLS.** The API can serve HTTPS directly (`--tls-cert`/`--tls-key`); plain HTTP
  on a non-loopback bind logs a warning. Put TLS in front of any exposed endpoint.
- **CORS.** Credentialed cross-origin access uses an **allowlist** — loopback
  origins always, others only via `--allowed-origin`. Arbitrary origins are never
  reflected.
- **CSRF.** State-changing cookie-authenticated requests require a matching CSRF
  token (constant-time compared). Bearer-token requests are exempt (not ambient).

## Cryptography

- **Secrets** (`capper secret`) — values encrypted with AES-256-GCM (random nonce
  per value) under a 32-byte master key. See [Secrets](../operator-guide/secrets.md).
- **KMS** (`capper kms`) — envelope encryption: data keys are generated and wrapped
  under the master key with AES-256-GCM. See [KMS](../operator-guide/kms.md).
- **Image signing & attestation** — Ed25519 image signatures (`capper sign`/
  `verify`/`keygen`), SBOM and provenance (`capper attest`/`sbom`), and posture
  scanning (`capper posture`). See
  [Posture, SBOM & signing](../operator-guide/posture-sbom-signing.md).

Master keys are files (`0600`) beside the data; protect the store directory.
Hardening the key root (passphrase / KMS-HSM-backed) is a roadmap item.

## Audit

Authorization decisions and sensitive operations are recorded in the audit log
(`capper iam audit`, `capper audit`), queryable per account and resource.

## Related

- [Projects & tenancy](projects.md) · [Manage IAM](../operator-guide/manage-iam.md)
  · [Secrets](../operator-guide/secrets.md) · [KMS](../operator-guide/kms.md)
  · [Posture, SBOM & signing](../operator-guide/posture-sbom-signing.md)
