---
title: "KMS"
description: "Envelope encryption with managed data keys wrapped under a master key."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# KMS

`capper kms` provides envelope encryption. A **data key** is generated, wrapped
(encrypted) under the master key with AES-256-GCM, and stored wrapped; you unwrap
it to encrypt/decrypt application data.

## Keys

```bash
capper kms key create my-key
capper kms key list
capper kms key rotate my-key       # rotate; old key retained for decrypt
capper kms key ...                 # encrypt / decrypt / inspect
```

## Envelope encryption pattern

1. Create a KMS key (the wrapping key).
2. Generate/derive a data key from it; KMS returns the data key plus its wrapped
   form.
3. Encrypt your data with the data key; store the wrapped data key alongside.
4. To read, unwrap the data key via KMS, then decrypt.

This keeps the long-lived secret (the master/wrapping key) out of your data path
and makes rotation cheap.

## Custody and rotation

By default the master (wrapping) key is a `0600` file beside the data — protect
the store directory. For passphrase-derived custody (no plaintext key on disk),
set `CAPPER_MASTER_PASSPHRASE`; see [Secrets → Key custody](secrets.md#key-custody).
Rotate data keys periodically; rotation keeps prior key material so existing
ciphertext still decrypts. HSM-backed custody remains a roadmap item.

## Related

- [Secrets](secrets.md) · [Security model](../concepts/security-model.md)
