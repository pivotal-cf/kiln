#!/bin/bash -eu

function main() {
  local cwd
  cwd="${1}"

  go run "${cwd}/../main.go" bake \
    --embed "${cwd}/extra" \
    --output-file "${cwd}/example-1.2.3-build.4.pivotal" \
    --variable "some-variable=some-value" \
    --variables-file "${cwd}/variables.yml" \
    --version "1.2.3-build.4" \
    --sha256
}

main "$(cd "$(dirname "${0}")" && pwd)"
