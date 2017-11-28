#!/bin/bash -eu

function main() {
  local cwd
  cwd="${1}"

  kiln bake \
    --embed "${cwd}/extra" \
    --forms-directory "${cwd}/forms" \
    --icon "${cwd}/icon.png" \
    --instance-groups-directory "${cwd}/instance-groups" \
    --metadata "${cwd}/base.yml" \
    --migrations-directory "${cwd}/migrations" \
    --output-file "${cwd}/example-1.2.3-build.4.pivotal" \
    --releases-directory "${cwd}/releases" \
    --runtime-configs-directory "${cwd}/runtime-configs" \
    --stemcell-tarball "${cwd}/stemcell.tgz" \
    --variables-directory "${cwd}/variables" \
    --version "1.2.3-build.4"
}

main "$(cd "$(dirname "${0}")" && pwd)"
