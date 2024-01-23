#!/bin/bash -exu

function main() {
  local cwd
  cwd="${1}"

  pushd "${cwd}" > /dev/null
    export CGO_ENABLED
    CGO_ENABLED=0

    go install \
      -ldflags "-X main.version=$(git rev-parse HEAD)" \
      -gcflags=-trimpath="${cwd}" \
      -asmflags=-trimpath="${cwd}" \
      ./
  popd > /dev/null
}

main "$(cd "$(dirname "${0}")" && pwd)"
