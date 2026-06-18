---
title: "Posture, SBOM & signing"
description: "Image posture scanning, SBOM/provenance attestation, and Ed25519 signing."
owner: "docs"
status: "stable"
reviewed: "2026-06-12"
outputs:
  - markdown
  - web
  - pdf
---

# Posture, SBOM & signing

Capper's image supply-chain security has three parts: **posture scanning**
(`capper posture`), **attestation** (`capper attest` — SBOM + provenance), and
**signing/verification** (`capper sign` / `verify` / `keygen`).

## Posture scanning

```bash
capper posture scan <image>      # scan for misconfigurations/vulnerabilities
capper posture list              # results / findings
```

Use posture checks as a gate in the [marketplace](marketplace.md) review flow and
in CI.

> **Vulnerability scanning requires [trivy](https://github.com/aquasecurity/trivy)
> on `PATH`.** The image `vuln` check shells out to `trivy fs`; when trivy is not
> installed the scan degrades to a non-fatal `warn` ("trivy not installed;
> vulnerability scan unavailable") rather than failing. Install trivy on any host
> that runs `capper posture scan` or image registry scans
> (`apt-get install trivy`, or see the trivy docs). The other checks (digest,
> signature, SBOM, secrets) have no external dependency.

## SBOM & provenance

```bash
capper attest sbom <image>        # generate a software bill of materials
capper attest provenance <image>  # generate build provenance
```

An SBOM records what is inside an image; provenance records how it was built. Store
them with the image so consumers can verify them.

## Signing & verification

```bash
capper keygen                    # generate an Ed25519 key pair
capper sign <image> --key priv.key
capper verify <image>            # verify the Ed25519 signature
```

Sign images you publish; verify before you run. Combine signing + posture + SBOM so
only vetted, signed images are admitted.

## A supply-chain gate

1. Build → `attest sbom` / `attest provenance`.
2. `posture scan` → fail the build on critical findings.
3. `sign` the image.
4. Publish to a [registry](registries.md) / [marketplace](marketplace.md).
5. `verify` (and check posture/SBOM) at admission.

## Related

- [Marketplace](marketplace.md) · [Registries](registries.md)
  · [Security model](../concepts/security-model.md)
