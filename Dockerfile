# syntax=docker/dockerfile:1
# =============================================================================
# ShipLog — read-only update advisor for Docker hosts (engine)
#
# GitHub: https://github.com/junkerderprovinz/shiplog
# Image:  ghcr.io/junkerderprovinz/shiplog
# License: MIT
#
# Pure-Go (cgo-free) → a single static binary on distroless: tiny always-on
# footprint for a 24/7 daemon. The build image must satisfy go.mod's directive
# (>= 1.25, pulled in by modernc.org/sqlite).
# =============================================================================
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/shiplog ./cmd/shiplog

# distroless/static ships CA certs (for the HTTPS registry/GitHub calls). It runs
# as root on purpose: ShipLog must READ /var/run/docker.sock (root-owned on
# Unraid) and write its SQLite db under /config. Security comes from the socket
# being mounted READ-ONLY (:ro) — even as root the container cannot write it; the
# binary never issues a non-GET Docker call.
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="ShipLog" \
      org.opencontainers.image.description="Read-only update advisor — what changes between your running image and the newest one, and how risky." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/shiplog" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /out/shiplog /usr/local/bin/shiplog
ENV PORT=8484 \
    DOCKER_SOCKET=/var/run/docker.sock \
    DATA_DIR=/config \
    POLL_INTERVAL=6h
EXPOSE 8484
VOLUME /config
ENTRYPOINT ["/usr/local/bin/shiplog"]
