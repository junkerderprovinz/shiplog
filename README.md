<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/shiplog-banner-dark.png">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/shiplog-banner.png" alt="ShipLog" width="100%">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/shiplog/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/shiplog/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/shiplog/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/shiplog/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=go&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/shiplog"><img src="https://img.shields.io/docker/pulls/junkerderprovinz/shiplog?style=for-the-badge&logo=docker&logoColor=white&label=Pulls&color=1d99f3" alt="Docker Pulls" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/shiplog"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/shiplog/latest?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=1d99f3" alt="Image Size" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/shiplog/pkgs/container/shiplog"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Plugin-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
⚓ <b>ShipLog</b> reads the changelog before you update — right inside Unraid's native <b>Docker tab</b>. Next to each container it shows <b>what actually changes</b> between your running image and the newest: the release notes, a deterministic <b>risk badge</b> (patch / minor / major) and the real <b>version jump</b> (e.g. 1.7 → 1.8).<br>
<br>
<b>Read-only</b> — it never pulls, recreates or stops anything. Optional: AI changelog summaries via a local Ollama, and Matrix notifications.
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
2. [Screenshots](#2-screenshots)
3. [Features](#3-features)
4. [Install on Unraid](#4-install-on-unraid)
5. [Configuration](#5-configuration)
6. [How it works](#6-how-it-works)
7. [Security](#7-security)
8. [License](#8-license)

## 1. What is this?

A single static Go binary on a distroless image (~tens of MB, low idle RAM) that polls the read-only Docker socket, resolves which images have updates, fetches the changelog for the version span, classifies the risk, and serves it on a small status page + JSON API. The headline experience — a changelog bubble next to each container in Unraid's Docker tab — ships as a companion Unraid plugin; this engine is the brain and works on any Docker host via its status page.

## 2. Screenshots

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/screenshots/changelog-bubble.png" alt="ShipLog changelog bubble in Unraid's Docker tab" width="90%">
  <br><em>Click the Changelog chip on any container — the bubble shows the version jump, a risk badge, the release notes, an optional AI summary, and a read-only Update-now button, right in Unraid's Docker tab.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/screenshots/docker-tab.png" alt="A Changelog chip on every container in the Docker tab" width="34%">
  <br><em>A Changelog chip sits on every container: a coloured dot when an update is waiting, grey when you're up to date.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/shiplog/main/.github/assets/screenshots/settings.png" alt="ShipLog settings page" width="62%">
  <br><em>Settings: poll interval, a live engine-status line, optional GitHub / Docker Hub tokens for more changelogs, Ollama (with autodetect) for AI summaries, and Matrix alerts.</em>
</p>

<br>

## 3. Features

- **What changed, not just "update available"** — changelog between your running tag and the newest, newest-first, with a link to the full release notes.
- **Deterministic risk badge** — digest/patch = low, minor = medium, major = high, non-semver = unknown (with a reason). Colour by default, with a colour ⇄ monochrome toggle.
- **Honest degradation** — when no changelog is machine-findable, ShipLog says so and shows what it does know, never pretends.
- **Read-only by construction** — never writes to the Docker socket.
- **Registry-friendly by construction** — manifest checks are `HEAD` requests, which do **not** count against Docker Hub's pull rate limit. Bearer tokens are cached, duplicate images share one lookup per sweep, and a rate-limiting registry is backed off host-wide instead of hammered.
- **Knows what has no upstream** — digest-pinned containers (`image@sha256:…`) and locally built images are labelled as such instead of producing bogus updates or permanent errors.
- **Update all in one click** — a counter button next to the Basic/Advanced toggle triggers Unraid's own bulk update for every container with a pending update (ShipLog itself stays read-only).
- **Update controls** — optional confirmation before an update, and an optional silent update that skips Unraid's pop-up download-log window (Settings → Updates).
- **Localised** — the settings page and the changelog bubble follow Unraid's configured language across 26 languages.
- **Optional, off by default:** AI changelog summaries via a local **Ollama**; enriched **Matrix** notifications.
- **Tiny + multi-arch** (amd64 + arm64), pure-Go (no cgo), boot-smoke-gated CI.

## 4. Install on Unraid

The Community Applications template is published in the [unraid-apps](https://github.com/junkerderprovinz/unraid-apps) feed (search **ShipLog** in the Apps tab once it lands). The only required mount is the Docker socket, read-only:

```
-v /var/run/docker.sock:/var/run/docker.sock:ro
-v /mnt/user/appdata/shiplog:/config
-p 8484:8484
```

Open the WebUI on port **8484**.

## 5. Configuration

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8484` | engine API + status page |
| `DOCKER_SOCKET` | `/var/run/docker.sock` | mounted read-only |
| `DATA_DIR` | `/config` | SQLite database + curated-mapping override |
| `POLL_INTERVAL` | `6h` | how often to re-check (e.g. `1h`, `12h`, `24h`) |
| `GITHUB_TOKEN` | — | optional; raises the anonymous GitHub API limit for changelog fetching |
| `OLLAMA_URL` / `OLLAMA_MODEL` | — | optional AI summaries |
| `MATRIX_HOMESERVER` / `MATRIX_TOKEN` / `MATRIX_ROOM` | — | optional enriched notifications |
| `CONFIRM_UPDATE` | `true` | ask for confirmation before "Update now" triggers Unraid's update |
| `SILENT_UPDATE` | `false` | run the update without Unraid's pop-up download-log window (it still runs in the background) |

## 6. How it works

Every `POLL_INTERVAL`, for each container:

1. **Discover** via the read-only Docker socket (image ref, digest, OCI labels). Digest-pinned (`image@sha256:…`), image-ID-referenced and locally built containers are recognised here and honestly labelled — they have no upstream to check, so no registry call is made for them.
2. **Resolve** the newest tag + same-tag digest from the registry (Docker Hub / GHCR / generic OCI v2, anonymous). Manifest checks are `HEAD`-only and don't consume Docker Hub's pull rate limit; the `tags/list` call is subject only to generic throttling, which ShipLog meets with per-request retries (honouring `Retry-After`), a per-host request gate, bearer-token caching, one lookup per distinct image per sweep, and a host-wide backoff after a hard 429.
3. **Changelog** via a layered provider chain — first hit wins: the image's `org.opencontainers.image.source` label → GitHub releases between the tags; otherwise a version-delta fallback with a compare link.
4. **Risk** is a pure function of the version delta.
5. **Store** in SQLite (status + a small per-container version history) and surface on the API + status page.

## 7. Security

ShipLog mounts the Docker socket **read-only** and never issues a write call — it cannot start, stop, recreate, or pull anything. v1 has no authentication and is intended for a trusted LAN; do not expose port 8484 to the internet. It makes outbound HTTPS calls to image registries and (for changelogs) GitHub.

## 8. License

MIT — see [LICENSE](LICENSE). ShipLog talks to upstream registries and GitHub over their public APIs; it bundles no third-party container content.

---

<sub>Part of a family of self-hosted Unraid apps + plugins by <b>junkerderprovinz</b> — see them all at <a href="https://github.com/junkerderprovinz">github.com/junkerderprovinz</a>, or install from <a href="https://unraid.net/community/apps">Community Applications</a>.</sub>
