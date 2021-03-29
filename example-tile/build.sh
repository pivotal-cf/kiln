#!/bin/bash -eu

function main() {
  local cwd
  cwd="${1}"

  go run "${cwd}/../main.go" bake \
    --kilnfile "${cwd}/Kilnfile" \
    --embed "${cwd}/extra" \
    --variable "some-variable=some-value" \
    --variables-file "${cwd}/variables.yml" \
    --sha256
}

main "$(cd "$(dirname "${0}")" && pwd)"
