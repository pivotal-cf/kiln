#!/bin/bash -eu

function main() {
  local cwd
  cwd="${1}"

  kiln bake \
    --releases-directory "${cwd}/releases" \
    --migrations-directory "${cwd}/migrations" \
    --runtime-configs-directory "${cwd}/runtime-configs" \
    --variables-directory "${cwd}/variables" \
    --embed "${cwd}/extra" \
    --stemcell-tarball "${cwd}/stemcell.tgz" \
    --metadata "${cwd}/base.yml" \
    --version "1.2.3-build.4" \
    --output-file "${cwd}/example-1.2.3-build.4.pivotal"
}

main "$(cd "$(dirname "${0}")" && pwd)"
