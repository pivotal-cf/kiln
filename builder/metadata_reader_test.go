package builder_test

import (
	"bytes"
	"errors"
	"io"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const HANDCRAFT = `---
metadata_version: '1.7'
provides_product_versions:
- name: cf
  version: 1.7.0.0
product_version: &product_version "1.2.3"
minimum_version_for_upgrade: 1.6.9-build.0
label: Pivotal Elastic Runtime
description:
  this is the description
rank: 90
serial: false
install_time_verifiers:
- name: Verifiers::SsoUrlVerifier
  properties:
    url: .properties.uaa.saml.sso_url
post_deploy_errands:
- name: smoke-tests
form_types:
- name: domains
  label: Domains
job_types:
- name: consul_server
  label: Consul
property_blueprints:
- name: product_version
  type: string
  configurable: false
  default: *product_version
`

var _ = Describe("MetadataReader", func() {
	var (
		filesystem *fakes.Filesystem
		logger     *fakes.Logger
		reader     builder.MetadataReader
	)

	BeforeEach(func() {
		filesystem = &fakes.Filesystem{}
		logger = &fakes.Logger{}
		reader = builder.NewMetadataReader(filesystem, logger)
	})

	Describe("Read", func() {
		It("parses the information from the metadata", func() {
			filesystem.OpenReturns(NewBuffer(bytes.NewBuffer([]byte(HANDCRAFT))), nil)

			metadata, err := reader.Read("/some/path/metadata.yml", "1.2.34-build.4")
			Expect(err).NotTo(HaveOccurred())

			Expect(filesystem.OpenArgsForCall(0)).To(Equal("/some/path/metadata.yml"))
			Expect(metadata["metadata_version"]).To(Equal("1.7"))
			Expect(metadata["provides_product_versions"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":    "cf",
				"version": "1.7.0.0",
			}}))
			Expect(metadata["product_version"]).To(Equal("1.2.3"))
			Expect(metadata["minimum_version_for_upgrade"]).To(Equal("1.6.9-build.0"))
			Expect(metadata["label"]).To(Equal("Pivotal Elastic Runtime"))
			Expect(metadata["description"]).To(Equal("this is the description"))
			Expect(metadata["rank"]).To(Equal(90))
			Expect(metadata["serial"]).To(BeFalse())
			Expect(metadata["install_time_verifiers"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name": "Verifiers::SsoUrlVerifier",
				"properties": map[interface{}]interface{}{
					"url": ".properties.uaa.saml.sso_url",
				},
			}}))
			Expect(metadata["post_deploy_errands"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name": "smoke-tests",
			}}))
			Expect(metadata["form_types"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":  "domains",
				"label": "Domains",
			}}))
			Expect(metadata["job_types"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":  "consul_server",
				"label": "Consul",
			}}))
			Expect(metadata["property_blueprints"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":         "product_version",
				"type":         "string",
				"configurable": false,
				"default":      "1.2.3",
			}}))
		})
	})

	Describe("failure cases", func() {
		Context("when the metadata file cannot be opened", func() {
			It("returns an error", func() {
				filesystem.OpenReturns(nil, errors.New("failed to open metadata"))

				_, err := reader.Read("/some/path/metadata.yml", "1.2.3-build.9999")
				Expect(err).To(MatchError("failed to open metadata"))
			})
		})

		Context("when the metadata file cannot be read", func() {
			It("returns an error", func() {
				erroringReader := &fakes.ReadCloser{}
				erroringReader.ReadReturns(0, errors.New("cannot read file"))
				filesystem.OpenStub = func(name string) (io.ReadCloser, error) {
					return erroringReader, nil
				}

				_, err := reader.Read("/some/path/metadata.yml", "1.2.3-build.9999")
				Expect(err).To(MatchError("cannot read file"))
				Expect(erroringReader.CloseCallCount()).To(Equal(1))
			})
		})

		Context("when the metadata yaml cannot be unmarshaled", func() {
			It("returns an error", func() {
				filesystem.OpenReturns(NewBuffer(bytes.NewBuffer([]byte("&&&&&&&&"))), nil)

				_, err := reader.Read("/some/path/metadata.yml", "1.2.3-build.9999")
				Expect(err).To(MatchError("yaml: did not find expected alphabetic or numeric character"))
			})
		})
	})
})
