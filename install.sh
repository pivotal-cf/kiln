#!/bin/bash -exu

function main() {
  local cwd
  cwd="${1}"

  pushd "${cwd}" > /dev/null
    go install \
      -ldflags "-X github.com/pivotal-cf/kiln/pkg/cargo.version=0.0.0-dev.$(git rev-parse HEAD | cut -c1-8).$(date +%s)" \
      -gcflags=-trimpath="${cwd}" \
      -asmflags=-trimpath="${cwd}" \
      ./
  popd > /dev/null
}

main "$(cd "$(dirname "${0}")" && pwd)"
