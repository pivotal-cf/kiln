#!/usr/bin/env bash

set -ex

# This script is for Generating the OSM manifests for the TAS GA Process.
# A release officer can run this script instead of the steps 2 & Bonus found at
# https://github.com/pivotal/tas/blob/main/.github/docs/GA_activities.md#osm

# This script assumes you have exported a Github Token, and will warn if it's missing.
# This script produces 3 OSM .yml Manifests for TAS, TASW, & IST. It will download and
# zip all packages specified in the Kilnfiles for the TAS flavors.

export WORKSPACE="${HOME}/workspace"

clone_tas(){
    if [ ! -d "$WORKSPACE/tas" ]; then
        git clone https://github.com/pivotal/tas "$WORKSPACE/tas"
    fi
}

generate_tas(){
    kiln generate-osm-manifest --github-token $GITHUB_TOKEN --kilnfile $WORKSPACE/tas/tas/Kilnfile > tas-osm.yml
    kiln generate-osm-manifest --github-token $GITHUB_TOKEN --only stack-auditor --url https://www.github.com/cloudfoundry/stack-auditor >> tas-osm.yml
}

generate_ist(){
    kiln generate-osm-manifest --github-token $GITHUB_TOKEN --kilnfile $WORKSPACE/tas/ist/Kilnfile > ist-osm.yml
}

generate_tasw(){
    kiln generate-osm-manifest --github-token $GITHUB_TOKEN --kilnfile $WORKSPACE/tas/tasw/Kilnfile > tasw-osm.yml
}

main() {
    if [ -z ${GITHUB_TOKEN} ]; then
        echo "Error: export your Github token to GITHUB_TOKEN"
        exit 1
    fi
    local git_token
    clone_tas
    generate_tas
    generate_ist
    generate_tasw
}

main "$@"
