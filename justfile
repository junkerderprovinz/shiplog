# Recipes use sh (Git Bash on Windows).

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

# Format check, vet, race tests and lint before a push
check:
    gofmt -l . | (! grep .) || (echo "gofmt: run 'just fmt'"; exit 1)
    go vet ./...
    go test -race ./...
    golangci-lint run --timeout 5m

# Build the Unraid plugin package (.txz), e.g. `just pkg 2.6.0`
pkg version="":
    bash plugin/pkg_build.sh {{version}}

# Secret-scan the working tree
secrets:
    gitleaks dir . --redact --no-banner
