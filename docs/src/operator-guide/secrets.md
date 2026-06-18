---
title: "Secrets"
description: "Store and retrieve encrypted key-value secrets (AES-256-GCM)."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Secrets

`capper secret` stores encrypted key-value secrets. Values are encrypted with
**AES-256-GCM** (a random nonce per value) under a 32-byte master key held `0600`
in the store.

## Manage secrets

```bash
capper secret create db-password --value "s3cr3t" --description "prod DB password"
capper secret list
capper secret inspect db-password    # metadata; value returned only to authorized callers
capper secret delete db-password
```

| Flag (`secret create`) | Purpose |
| --- | --- |
| `--value <string>` | the plaintext secret value (required) |
| `--description <text>` | optional description shown in listings |

Inject a secret into an instance at launch with
`capper run --secret db-password=DATABASE_PASSWORD …` (see
[Manage instances](manage-instances.md)).

## Access control

Secrets are governed by [IAM](manage-iam.md) like any other resource, and scoped
to a project/account. Grant least-privilege access to the principals (and service
accounts) that need them.

## Key custody

By default the AES-256 master key is a `0600` file beside the data, so protect the
store directory.

For stronger custody, set **`CAPPER_MASTER_PASSPHRASE`** (e.g. via a systemd
credential): the master keys for secrets, KMS, and the IAM token-signing key are
then derived from that passphrase via PBKDF2-HMAC-SHA256 (600k iterations) with a
per-key salt, and **no plaintext key is written to disk** — the root of trust
moves off the disk to the runtime-supplied passphrase.

> Migration note: enabling the passphrase on a deployment that already has
> file-based keys is refused (it would orphan existing ciphertext). Start with the
> passphrase set, or re-encrypt deliberately. HSM-backed custody remains a roadmap
> item.

For envelope encryption of application data keys, use [KMS](kms.md).

## Related

- [KMS](kms.md) · [Security model](../concepts/security-model.md) · [Manage IAM](manage-iam.md)
