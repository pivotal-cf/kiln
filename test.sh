#!/bin/bash

set -euxo pipefail

export CGO_ENABLED
CGO_ENABLED=0

function main() {
  local cwd
  cwd="${1}"

  pushd "${cwd}" > /dev/null
    export CGO_ENABLED
    CGO_ENABLED=0

    go test -cover -short ./...
    go vet ./...

    golangci-lint run ./...

    set +x
    echo "Setting GITHUB_TOKEN with 'gh auth token'"
    export GITHUB_TOKEN
    GITHUB_TOKEN="$(gh auth token)"
    set -x

    go test -v -count=1 -tags acceptance --timeout=25m ./internal/acceptance/workflows
  popd > /dev/null
}

main "$(cd "$(dirname "${0}")" && pwd)"
