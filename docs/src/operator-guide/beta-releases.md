# Publishing Beta Releases

Capper AIO artifacts are generated under `DIST/AIO/`. That directory is
ignored by git because the tarballs are large release outputs, not source
files. Publish them to GitHub Releases instead.

## Build the AIO matrix

```bash
rm -rf DIST/AIO/*
SKIP_TESTS=1 scripts/release-matrix.sh 0.1.38
```

This produces one `.tgz` and one `.sha256` file per target platform, plus
`channels.json`.

## Publish a prerelease

Install and authenticate the GitHub CLI first:

```bash
gh auth login
```

Then upload all AIO artifacts as a beta prerelease:

```bash
scripts/github-release-beta.sh 0.1.38 1
```

The command creates or updates:

```text
v0.1.38-beta.1
```

Uploaded assets include every `capper-aio-*.tgz`, every matching
`capper-aio-*.tgz.sha256`, and `DIST/AIO/channels.json`.

## Install from a release asset

```bash
sha256sum -c capper-aio-0.1.38-ubuntu24.04-glibc2.39-x86_64.tgz.sha256
tar xzf capper-aio-0.1.38-ubuntu24.04-glibc2.39-x86_64.tgz
cd capper-aio-0.1.38-ubuntu24.04-glibc2.39-x86_64
sudo ./install.sh --check-only
sudo ./install.sh --yes
sudo capper aio doctor
sudo capper aio init --backend capdb
sudo capper aio up
```
