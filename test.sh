#!/bin/bash

set -euxo pipefail

function main() {
  local cwd
  cwd="${1}"

  pushd "${cwd}" > /dev/null
    go test -cover -short ./...
    go vet ./...

    golangci-lint run ./...

    set +x
    echo "Setting GITHUB_TOKEN with 'gh auth token'"
    export GITHUB_TOKEN
    GITHUB_TOKEN="$(gh auth token)"
    set -x

    go test -tags acceptance --timeout=25m ./internal/acceptance/workflows
  popd > /dev/null
}

main "$(cd "$(dirname "${0}")" && pwd)"
