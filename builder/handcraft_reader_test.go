package builder_test

import (
	"bytes"
	"errors"

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
product_version: &product_version "1.7.0.0$PRERELEASE_VERSION$"
minimum_version_for_upgrade: 1.6.9-build.0
label: Pivotal Elastic Runtime
description:
  this is the description
icon_image: some-image
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

var _ = Describe("HandcraftReader", func() {
	var (
		filesystem *fakes.Filesystem
		logger     *fakes.Logger
		reader     builder.HandcraftReader
	)

	BeforeEach(func() {
		filesystem = &fakes.Filesystem{}
		logger = &fakes.Logger{}
		reader = builder.NewHandcraftReader(filesystem, logger)
	})

	Describe("Read", func() {
		It("parses the information from the handcraft", func() {
			filesystem.OpenCall.Returns.File = NewBuffer(bytes.NewBuffer([]byte(HANDCRAFT)))

			handcraft, err := reader.Read("/some/path/handcraft.yml", "1.2.34-build.4")
			Expect(err).NotTo(HaveOccurred())

			Expect(filesystem.OpenCall.Receives.Path).To(Equal("/some/path/handcraft.yml"))
			Expect(handcraft["metadata_version"]).To(Equal("1.7"))
			Expect(handcraft["provides_product_versions"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":    "cf",
				"version": "1.7.0.0",
			}}))
			Expect(handcraft["product_version"]).To(Equal("1.2.34-build.4"))
			Expect(handcraft["minimum_version_for_upgrade"]).To(Equal("1.6.9-build.0"))
			Expect(handcraft["label"]).To(Equal("Pivotal Elastic Runtime"))
			Expect(handcraft["description"]).To(Equal("this is the description"))
			Expect(handcraft["icon_image"]).To(Equal("some-image"))
			Expect(handcraft["rank"]).To(Equal(90))
			Expect(handcraft["serial"]).To(BeFalse())
			Expect(handcraft["install_time_verifiers"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name": "Verifiers::SsoUrlVerifier",
				"properties": map[interface{}]interface{}{
					"url": ".properties.uaa.saml.sso_url",
				},
			}}))
			Expect(handcraft["post_deploy_errands"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name": "smoke-tests",
			}}))
			Expect(handcraft["form_types"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":  "domains",
				"label": "Domains",
			}}))
			Expect(handcraft["job_types"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":  "consul_server",
				"label": "Consul",
			}}))
			Expect(handcraft["property_blueprints"]).To(Equal([]interface{}{map[interface{}]interface{}{
				"name":         "product_version",
				"type":         "string",
				"configurable": false,
				"default":      "1.2.34-build.4",
			}}))

			Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
				"Injecting version \"1.2.34-build.4\" into handcraft...",
			}))
		})
	})

	Describe("failure cases", func() {
		Context("when the handcraft file cannot be opened", func() {
			It("returns an error", func() {
				filesystem.OpenCall.Returns.Error = errors.New("failed to open handcraft")

				_, err := reader.Read("/some/path/handcraft.yml", "1.2.3-build.9999")
				Expect(err).To(MatchError("failed to open handcraft"))
			})
		})

		Context("when the handcraft file cannot be read", func() {
			It("returns an error", func() {
				file := NewBuffer(bytes.NewBuffer([]byte(HANDCRAFT)))
				file.Error = errors.New("failed to read")
				filesystem.OpenCall.Returns.File = file

				_, err := reader.Read("/some/path/handcraft.yml", "1.2.3-build.9999")
				Expect(err).To(MatchError("failed to read"))
			})
		})

		Context("when the handcraft yaml cannot be unmarshaled", func() {
			It("returns an error", func() {
				filesystem.OpenCall.Returns.File = NewBuffer(bytes.NewBuffer([]byte("&&&&&&&&")))

				_, err := reader.Read("/some/path/handcraft.yml", "1.2.3-build.9999")
				Expect(err).To(MatchError("yaml: did not find expected alphabetic or numeric character"))
			})
		})
	})
})
