---
title: "Registries"
description: "Image and artifact registries: push, pull, tokens, and garbage collection."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Registries

`capper registry` manages image and artifact registries that store `.cap` images
and other build artifacts.

## Manage

```bash
capper registry init my-reg
capper registry list
capper registry inspect my-reg
capper registry image ...        # manage images
capper registry artifact ...     # manage non-image artifacts
capper registry token ...        # access tokens for push/pull
capper registry gc               # garbage-collect unreferenced blobs
capper registry delete my-reg
```

## Access

Push/pull is authenticated with registry tokens; scope them to least privilege.
Run `registry gc` periodically to reclaim space from deleted/untagged content.

## Related

- [Marketplace](marketplace.md) · [Posture, SBOM & signing](posture-sbom-signing.md)
  · [Manage instances](manage-instances.md)
