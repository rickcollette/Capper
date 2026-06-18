---
title: "Marketplace"
description: "Submit, review, scan, and install marketplace images."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Marketplace

`capper market` is the image marketplace with a review/approval workflow and
security scanning.

## Publish and review

```bash
capper market submit ./my-image.cap ...    # submit for review
capper market scan <listing>               # run a posture/security scan
capper market inspect <listing>
capper market approve <listing>            # approve for distribution
capper market reject <listing>
```

Submissions are scanned (see [posture & SBOM](posture-sbom-signing.md)) and must
be approved before they are installable.

## Install

```bash
capper market list
capper market install <listing>
```

## Trust

Combine marketplace review with [image signing](posture-sbom-signing.md) and
[posture scanning](posture-sbom-signing.md) so only vetted, signed images run.

## Related

- [Posture, SBOM & signing](posture-sbom-signing.md) · [Registries](registries.md)
