package acceptance_test

var expectedMetadata = `---
description: this is the description
some_forms:
- description: some-other-form-description
  label: some-other-form-label
  name: some-other-config
- description: some-form-description
  label: some-form-label
  name: some-config
- description: some-form-description
  label: some-form-label
  name: some-more-config
icon_img: aS1hbS1zb21lLWltYWdl
install_time_verifiers:
- name: Verifiers::SsoUrlVerifier
  properties:
    url: .properties.uaa.saml.sso_url
some_job_types:
- label: Some Instance Group
  name: some-instance-group
  templates:
  - name: some-job
    release: some-release
- label: Some Other Instance Group
  name: some-other-instance-group
  templates:
  - name: some-other-job
    release: some-other-release
label: Pivotal Elastic Runtime
metadata_version: "1.7"
minimum_version_for_upgrade: 1.6.9-build.0
custom_variable: some-variable-value
literal_variable: |
  { "some": "value" }
boolean_variable: true
some_bosh_variables:
- name: variable-1
  type: certificate
  options:
    some_option: Option value
- name: variable-2
  type: password
name: cool-product-name
post_deploy_errands:
- name: smoke-tests
product_version: 1.2.3
some_property_blueprints:
- name: some_templated_property_blueprint
  type: boolean
  configurable: false
  default: true
provides_product_versions:
- name: cf
  version: 1.7.0.0
rank: 90
some_releases:
- file: diego-release-0.1467.1-3215.4.0.tgz
  name: diego
  version: 0.1467.1
  sha1: %s
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
some_stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
some_runtime_configs:
- name: some-runtime-config
  runtime_config: |
    releases:
    - name: some-addon
      version: some-addon-version
serial: false
selected_value: "235"
`

var expectedMetadataWithStemcellCriteria = `---
icon_img: aS1hbS1zb21lLWltYWdl
label: Pivotal Elastic Runtime
metadata_version: "1.7"
name: cool-product-name
product_version: 1.2.3
some_releases:
- file: diego-release-0.1467.1-3215.4.0.tgz
  name: diego
  version: 0.1467.1
  sha1: %s
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
stemcell_criteria:
  os: ubuntu-xenial
  version: 250.21
  requires_cpi: false
  enable_patch_security_updates: true
`

var expectedMetadataWithMultipleStemcells = `---
icon_img: aS1hbS1zb21lLWltYWdl
label: Pivotal Elastic Runtime
metadata_version: "1.7"
name: cool-product-name
product_version: 1.2.3
some_releases:
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
additional_stemcells_criteria:
- os: windows
  version: "2019.4"
`

var expectedMetadataWithStemcellTarball = `---
icon_img: aS1hbS1zb21lLWltYWdl
label: Pivotal Elastic Runtime
metadata_version: "1.7"
name: cool-product-name
product_version: 1.2.3
some_releases:
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
`

