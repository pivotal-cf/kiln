#!/usr/bin/env bash

set -euo pipefail

export CGO_ENABLED
CGO_ENABLED=0

function main() {
  local cwd
  cwd="${1}"

  pushd "${cwd}" > /dev/null
    export CGO_ENABLED
    CGO_ENABLED=0

    >&2 echo "Running unit tests..."
    go test -cover -short ./...

    >&2 echo "Running go vet..."
    go vet ./...

    >&2 echo "Running golangci-lint run..."
    golangci-lint run ./...

    >&2 echo "Setting GITHUB_ACCESS_TOKEN with 'gh auth token'..."
    export GITHUB_ACCESS_TOKEN
    GITHUB_ACCESS_TOKEN="$(gh auth token)"

    >&2 echo "Running acceptance tests..."
    go test -v -count=1 -tags acceptance --timeout=25m ./internal/acceptance/workflows
  popd > /dev/null
}

main "$(cd "$(dirname "$(dirname "${0}")")" && pwd)"
