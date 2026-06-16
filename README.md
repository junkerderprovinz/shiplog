<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/shiplog-banner.png" alt="ShipLog" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/shiplog/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/shiplog/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/shiplog/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/shiplog/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=go&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/shiplog/pkgs/container/shiplog"><img src="https://img.shields.io/badge/Image-ghcr.io-1d99f3?style=for-the-badge&logo=docker&logoColor=white" alt="Image" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/shiplog/pkgs/container/shiplog"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Plugin-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
⚓ <b>ShipLog</b> is a tiny, <b>read-only update advisor</b> for Docker hosts. Unraid (and the CA auto-update plugin) already tell you <i>that</i> an update is available and let you apply it — ShipLog tells you <b>what actually changes</b> and <b>how risky it is</b> first: the changelog between your running image and the newest one, a deterministic risk verdict, and an optional AI summary.<br>
<br>
It never pulls, recreates, or stops anything. The Docker socket is read-only.
</p>

<br>

<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" width="220">
  </a>
</p>

<br>

## Table of Contents

1. [What is this?](#1-what-is-this)
2. [Features](#2-features)
3. [Install on Unraid](#3-install-on-unraid)
4. [Configuration](#4-configuration)
5. [How it works](#5-how-it-works)
6. [Security](#6-security)
7. [License](#7-license)

## 1. What is this?

A single static Go binary on a distroless image (~tens of MB, low idle RAM) that polls the read-only Docker socket, resolves which images have updates, fetches the changelog for the version span, classifies the risk, and serves it on a small status page + JSON API. The headline experience — a changelog bubble next to each container in Unraid's Docker tab — ships as a companion Unraid plugin; this engine is the brain and works on any Docker host via its status page.

## 2. Features

- **What changed, not just "update available"** — changelog between your running tag and the newest, newest-first, with a link to the full release notes.
- **Deterministic risk badge** — digest/patch = low, minor = medium, major = high, non-semver = unknown (with a reason). Colour by default, with a colour ⇄ monochrome toggle.
- **Honest degradation** — when no changelog is machine-findable, ShipLog says so and shows what it does know, never pretends.
- **Read-only by construction** — never writes to the Docker socket.
- **Optional, off by default:** AI changelog summaries via a local **Ollama**; enriched **Matrix** notifications.
- **Tiny + multi-arch** (amd64 + arm64), pure-Go (no cgo), boot-smoke-gated CI.

## 3. Install on Unraid

The Community Applications template is published in the [unraid-docker-templates](https://github.com/junkerderprovinz/unraid-docker-templates) feed (search **ShipLog** in the Apps tab once it lands). The only required mount is the Docker socket, read-only:

```
-v /var/run/docker.sock:/var/run/docker.sock:ro
-v /mnt/user/appdata/shiplog:/config
-p 8484:8484
```

Open the WebUI on port **8484**.

## 4. Configuration

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8484` | engine API + status page |
| `DOCKER_SOCKET` | `/var/run/docker.sock` | mounted read-only |
| `DATA_DIR` | `/config` | SQLite database + curated-mapping override |
| `POLL_INTERVAL` | `6h` | how often to re-check (e.g. `1h`, `12h`, `24h`) |
| `GITHUB_TOKEN` | — | optional; raises the anonymous GitHub API limit for changelog fetching |
| `OLLAMA_URL` / `OLLAMA_MODEL` | — | optional AI summaries |
| `MATRIX_HOMESERVER` / `MATRIX_TOKEN` / `MATRIX_ROOM` | — | optional enriched notifications |

## 5. How it works

Every `POLL_INTERVAL`, for each container:

1. **Discover** via the read-only Docker socket (image ref, digest, OCI labels).
2. **Resolve** the newest tag + same-tag digest from the registry (Docker Hub / GHCR / generic OCI v2, anonymous).
3. **Changelog** via a layered provider chain — first hit wins: the image's `org.opencontainers.image.source` label → GitHub releases between the tags; otherwise a version-delta fallback with a compare link.
4. **Risk** is a pure function of the version delta.
5. **Store** in SQLite (status + a small per-container version history) and surface on the API + status page.

## 6. Security

ShipLog mounts the Docker socket **read-only** and never issues a write call — it cannot start, stop, recreate, or pull anything. v1 has no authentication and is intended for a trusted LAN; do not expose port 8484 to the internet. It makes outbound HTTPS calls to image registries and (for changelogs) GitHub.

## 7. License

MIT — see [LICENSE](LICENSE). ShipLog talks to upstream registries and GitHub over their public APIs; it bundles no third-party container content.
