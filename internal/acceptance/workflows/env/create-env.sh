#! /usr/bin/env bash
# Uses terraforming-gcp-releng: https://github.com/pivotal/terraforming-gcp-releng#readme
# TODO: check for / require gcloud, terraform at very old version (0.11.15)

set -eu pipefail

if [ ! -d ./terraforming-gcp-releng ]; then
  git clone git@github.com:pivotal/terraforming-gcp-releng.git
fi

if [ ! -f ./bin/terraform ]; then
  mkdir -p bin
  pushd bin >> /dev/null || exit 72
    curl -fL -o terraform.zip https://releases.hashicorp.com/terraform/0.11.15/terraform_0.11.15_darwin_amd64.zip
    unzip terraform.zip
    chmod +x terraform

    ./terraform --version
  popd
fi

export TF_VAR_env_name='fhloston-paradise'
export TF_VAR_project='cf-sandbox-release-engineering'
export TF_VAR_region='us-west1'
export TF_VAR_zones='["us-west1-a", "us-west1-b", "us-west1-c"]'
export TF_VAR_dns_suffix="kiln.releng.cf-app.com"
# From https://network.pivotal.io/products/ops-manager/
export TF_VAR_opsman_image="ops-manager-2-10-44-build-502"
export TF_VAR_ssl_ca_cert="$(vault read -field certificate runway_concourse/ppe-ci/releng_ca)"
export TF_VAR_ssl_ca_private_key="$(vault read -field private_key runway_concourse/ppe-ci/releng_ca)"
export TF_VAR_service_account_key="$(vault read --field json_key runway_concourse/ppe-ci/kiln_acceptance_gcp_key)"
export TF_VAR_global_lb='0'

pushd terraforming-gcp-releng/terraforming-pas >> /dev/null || exit 36
  ../../bin/terraform init
  ../../bin/terraform plan -out=plan
  ../../bin/terraform apply plan
popd
