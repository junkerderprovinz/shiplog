# ShipLog — the Unraid plugin (bubble in the native Docker tab)

ShipLog is delivered as **one slim Unraid plugin**. The plugin bundles the Go
**engine** and runs it as a host daemon (no separate Docker container), and it
injects a discreet changelog control + bubble into Unraid's **native Docker tab**.

| Part | What it is | How it runs |
|------|------------|-------------|
| **Engine** | The read-only Go binary: reads the Docker socket, resolves running→newest, computes the risk, fetches the changelog, serves a small REST API + status page. | Started by the plugin as a host daemon (`scripts/rc.shiplog`). Same binary as the container image — the container is still buildable for non-Unraid hosts. |
| **Injection** | A `*.Docker.page` that emhttp renders into the Docker tab and that loads `scripts/docker.js` + `styles/docker.css`. | In the browser, on the Docker tab. |
| **Proxy** | `server/status.php` — same-origin bridge from the browser to the local engine (`127.0.0.1:<port>`), so there's no CORS and no token in the browser. | On the Unraid webserver. |

**Why a plugin (and why the engine isn't separate PHP).** Unraid's Docker tab is
server-rendered by emhttp; only a plugin can draw into it. The data work
(registry/OCI calls, changelog provider chain, risk, optional Ollama/Matrix) is
the already-built, tested Go engine — so the plugin ships and runs that binary
rather than reimplementing it in PHP. Precedent for the injection:
[scolcipitato/folder.view](https://github.com/scolcipitato/folder.view) uses the
same `folder.view.Docker.page` → `scripts/docker.js` + `server/*.php` pattern.

## Layout

```
plugin/
  shiplog.plg                 installer: fetch the .txz from the GitHub release, start the daemon
  pkg_build.sh                build the engine binary + package the .txz (+ sha256)
  spike/docker-tab-inject.js  the P2.0 feasibility spike (paste-in-console; proven on a real box)
  src/shiplog/usr/local/emhttp/plugins/shiplog/
    shiplog.Docker.page       Menu="Docker" → injected into the Docker tab
    ShipLog.page              Settings → ShipLog (port, enable, token, Ollama/Matrix)
    scripts/docker.js         the injection: proxy fetch → per-row chip + bubble
    styles/docker.css         the chip + bubble styles
    server/status.php         same-origin proxy → the local engine
    scripts/rc.shiplog        daemon control (start/stop/restart/status, PID file)
    event/started             array start → start the daemon
    event/stopping_svcs       array stop → stop the daemon
    bin/shiplog               the engine binary (added by pkg_build.sh)
```

- **Config** persists on flash at `/boot/config/plugins/shiplog/shiplog.cfg`
  (written by the settings page; read by the proxy + the daemon).
- **Data** (SQLite cache) lives in appdata (`DATA_DIR`, default
  `/mnt/user/appdata/shiplog`) — never on the flash.
- The chip appears **only for containers with an update**; up-to-date rows are
  left untouched. If the engine is down, nothing renders and the Docker tab is
  unaffected (the engine status page `http://<host>:<port>/` is the fallback).

## Build + release

```bash
plugin/pkg_build.sh 2026.06.15            # → plugin/out/shiplog-2026.06.15-x86_64-1.txz (+ .sha256)
```

The release workflow attaches the `.txz` to a `plugin-<version>` GitHub release
and injects its SHA256 into `shiplog.plg`. Users install by pasting the raw
`shiplog.plg` URL into Unraid **Plugins → Install Plugin** (later: the CA plugins
section).

## P2.0 — feasibility spike (already proven on a real box)

[`spike/docker-tab-inject.js`](spike/docker-tab-inject.js) is the throwaway proof
that the DOM hook works (paste into the Docker tab's DevTools console). It was
run on Bottich and confirmed the injection + the update-column placement. The
production `scripts/docker.js` is the same approach wired to the engine via the
proxy.
