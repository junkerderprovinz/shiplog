# Your own apps on Unraid with ShipLog

Two things read a container's setup, and both go wrong when a field is off:

- **Unraid's update column** (up-to-date, update ready, not available). It asks the registry for the digest behind the image tag, with the logins in `/root/.docker/config.json`. If it gets no digest, it shows *not available*.
- **ShipLog's badges** (the changelog chip, the red **Discontinued** and the amber **Deprecated** badge). ShipLog works them out again on every check, so once the cause is fixed the badge goes away on the next one.

Go through these points for every app you build or publish yourself.

1. **Decide whether the image is public or private.** A public package (on GHCR under *Package settings*, on Docker Hub or Gitea a public repository) can be read without a login, and nothing else is needed. A private one needs the login from point 2. Without it, a private GHCR package answers 401 or 403; Unraid then shows *not available*, and ShipLog treats it as "cannot see", never as "removed".
2. **For a private image, log Unraid in to the registry.** As root: `printf '%s' "$TOKEN" | docker login <registry-host> -u <user> --password-stdin` (for GHCR a classic token with `read:packages`). The key under `auths` has to be exactly the host in the template's `<Repository>`, such as `ghcr.io` or `registry.example.org`, and the login has to be stored in the file itself: Unraid and ShipLog both ignore `credsStore` and `credHelpers`. Unraid keeps `/root` in RAM, so copy the file to the flash drive and restore it from `/boot/config/go`, or the login is gone after a reboot. ShipLog reads the same file (`DOCKER_CONFIG`, `/root/.docker` on Unraid), so there is no second login to keep in sync. When the token expires, the column quietly goes back to *not available*, so note the date.
3. **Put the full, pullable reference in `<Repository>`, and push that tag.** For example `ghcr.io/<you>/<app>:latest`. An image that only exists on the server counts as built locally and is not checked. Do not delete or rename a package while a container still runs from it: once the registry answers *repository unknown*, ShipLog correctly reports **Image no longer in the registry**.
4. **Leave `<TemplateURL>` empty unless you publish the template, and never reuse a Community Applications path.** An empty field means there is nothing to check. If you publish a template, point at the raw file in your own repository (`https://raw.githubusercontent.com/<you>/<repo>/<branch>/<path>.xml`, not a github.com `blob` page) and keep the path stable. ShipLog only gives Community Applications verdicts to templates from a repository CA crawls, according to the feed's own list, so a template in your own repository, public or private, is never "Removed from Community Applications". Once you submit the repository to CA, the app is treated like any other CA app.
5. **Set the OCI labels in the image.** `org.opencontainers.image.source=https://github.com/<you>/<repo>` tells ShipLog where the release notes are, and GHCR links the package to that repository. If that repository is archived, ShipLog shows **Discontinued**, so point the label at the live one. `org.opencontainers.image.version` lets a `:latest` container show its version straight away. Unraid adds `net.unraid.docker.managed` itself when it creates a container from a template; create your apps from a template rather than with `docker run` or Compose if the "Ignore third-party containers" setting should keep them.
6. **Read a badge's reason before you work around it.** *Removed from Community Applications* means CA blacklisted the app or dropped it from a repository it crawls. *Image no longer in the registry* means the registry said "repository unknown". *Source repository archived* means the repository in the source label is archived. A changelog source override or hiding the badge is fine when the cause is understood; for a mistake in the setup, fixing the setup is better.
7. **Check once after the setup.** Replace the placeholders:
   ```sh
   # 200 means the anonymous token was issued, so the image is public; 401 or 403 means private or missing
   curl -s -o /dev/null -w '%{http_code}\n' "https://ghcr.io/token?service=ghcr.io&scope=repository:<you>/<app>:pull"
   # Unraid's view: a status and a remote digest mean the login works
   jq '."ghcr.io/<you>/<app>:latest"' /var/lib/docker/unraid-update-status.json
   # ShipLog's view: no "unmaintained" field and a kind of none or an update
   curl -s http://<server>:8484/api/containers | jq '.[] | select(.container.name=="<App>") | {kind, unmaintained, unmaintained_reason}'
   ```
