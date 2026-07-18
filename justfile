# ShipLog task runner — run `just` to list recipes.
# Recipes use sh (Git Bash on Windows). See CLAUDE.md for the full guide.

# List available recipes
default:
    @just --list

# Compile everything
build:
    go build ./...

# Run the full test suite with the race detector (as CI does)
test:
    go test -race ./...

# Format all Go code in place
fmt:
    gofmt -w .

# Format check + vet + race tests + lint + Dockerfile lint (full pre-push chain)
check:
    gofmt -l . | (! grep .) || (echo "gofmt: run 'just fmt'"; exit 1)
    go vet ./...
    go test -race ./...
    golangci-lint run --timeout 5m
    hadolint Dockerfile

# Build the engine Docker image locally
image:
    docker build -t shiplog:dev .

# Build the Unraid plugin package (.txz) — e.g. `just pkg 2.6.0`
pkg version="":
    bash plugin/pkg_build.sh {{version}}

# Secret-scan the working tree
secrets:
    gitleaks dir . --redact --no-banner
