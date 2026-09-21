# syntax=docker/dockerfile:1@sha256:ecfaec9ed6d810b56388c508f4121597bfbba70d41a6dfeee4d8cad5f295fc32
# ShipLog engine, a read-only update advisor for Docker hosts.
# https://github.com/junkerderprovinz/shiplog, AGPL-3.0-only
#
# The build image has to satisfy the go directive in go.mod, which
# modernc.org/sqlite raises to 1.25.
FROM golang:1.27-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/shiplog ./cmd/shiplog

# The base image ships the CA certificates the registry and GitHub calls need.
# It runs as root because the Docker socket is root-owned on Unraid; the socket
# is mounted read-only, and the binary only issues GET requests to it. Renovate
# keeps the pinned digest current.
FROM gcr.io/distroless/static-debian12:latest@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2
LABEL org.opencontainers.image.title="ShipLog" \
      org.opencontainers.image.description="Read-only update advisor: what changes between your running image and the newest one, and how risky." \
      org.opencontainers.image.source="https://github.com/junkerderprovinz/shiplog" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY --from=build /out/shiplog /usr/local/bin/shiplog
ENV PORT=8484 \
    DOCKER_SOCKET=/var/run/docker.sock \
    DATA_DIR=/config \
    POLL_INTERVAL=6h
EXPOSE 8484
VOLUME /config
ENTRYPOINT ["/usr/local/bin/shiplog"]
