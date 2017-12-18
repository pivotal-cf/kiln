package acceptance

var untemplatedMetadata = []byte(`---
name: cool-product-name
metadata_version: '1.7'
releases:
  - $( release "diego" )
  - $( release "cf" )
stemcell_criteria: $( stemcell )
provides_product_versions:
- name: cf
  version: 1.7.0.0
product_version: "1.7.0.0$PRERELEASE_VERSION$"
minimum_version_for_upgrade: 1.6.9-build.0
custom_variable: $(variable "some-variable")
label: Pivotal Elastic Runtime
description:
  this is the description
icon_img: $( icon )
rank: 90
serial: false
install_time_verifiers:
- name: Verifiers::SsoUrlVerifier
  properties:
    url: .properties.uaa.saml.sso_url
post_deploy_errands:
- name: smoke-tests
form_types:
- $( form "some-other-config" )
- $( form "some-config" )
- $( form "some-more-config" )
job_types:
- name: consul_server
  label: Consul
`)

var expectedMetadata = `---
description: this is the description
form_types:
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
job_types:
- label: Some Other Instance Group
  name: some-other-instance-group
  templates:
  - name: some-other-job
    release: some-other-release
- label: Some Instance Group
  name: some-instance-group
  templates:
  - name: some-job
    release: some-release
label: Pivotal Elastic Runtime
metadata_version: "1.7"
minimum_version_for_upgrade: 1.6.9-build.0
custom_variable: some-variable-value
name: cool-product-name
post_deploy_errands:
- name: smoke-tests
product_version: 1.2.3
property_blueprints:
- name: some_property_blueprint
  type: boolean
  configurable: true
  default: false
provides_product_versions:
- name: cf
  version: 1.7.0.0
rank: 90
releases:
- file: diego-release-0.1467.1-3215.4.0.tgz
  name: diego
  version: 0.1467.1
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
runtime_configs:
- name: some_addon
  runtime_config: |
    releases:
    - name: some-addon
serial: false
variables:
- name: variable-1
  options:
    some_option: Option value
  type: certificate
- name: variable-2
  type: password
`
