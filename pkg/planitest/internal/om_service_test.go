package internal_test

import (
	"errors"
	"io"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/planitest/internal"
	"github.com/pivotal-cf/kiln/pkg/planitest/internal/fakes"
)

var _ = Describe("OM Service", func() {
	var (
		omRunner *fakes.OMRunner

		omService *internal.OMService

		tileConfig   io.Reader
		tileMetadata io.Reader
	)

	BeforeEach(func() {
		omRunner = &fakes.OMRunner{}
		var err error

		omService, err = internal.NewOMServiceWithRunner(omRunner)
		Expect(err).NotTo(HaveOccurred())

		tileConfig = strings.NewReader(`{
						"network-properties":{
							"network":{
								"name":"some-network"
							}
						},
						"product-properties": {
							".some-minimal-config":{
								"value":"some-value"
							},
							"some.additional.property": {
								"value":"some-value"
							}
						}
					}`)

		tileMetadata = strings.NewReader(`
---
name: cf
product_version: 1.2.3
`)
	})

	Describe("RenderManifest", func() {
		BeforeEach(func() {
			omRunner.FindStagedProductReturnsOnCall(0, internal.StagedProduct{
				GUID:           "some-guid",
				Type:           "some-type",
				ProductVersion: "some-version",
			}, nil)
			omRunner.FindStagedProductReturnsOnCall(1, internal.StagedProduct{
				GUID:           "some-updated-guid",
				Type:           "some-type",
				ProductVersion: "some-version",
			}, nil)
			omRunner.GetManifestReturns(map[string]any{
				"some-key": "some-value",
			}, nil)
		})

		It("calls OM Runner to retrieve the BOSH manifest", func() {
			manifest, err := omService.RenderManifest(tileConfig, tileMetadata)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest).To(MatchYAML(`some-key: some-value`))

			Expect(omRunner.FindStagedProductCallCount()).To(Equal(2))
			productName := omRunner.FindStagedProductArgsForCall(0)
			Expect(productName).To(Equal("cf"))
			productName = omRunner.FindStagedProductArgsForCall(1)
			Expect(productName).To(Equal("cf"))

			Expect(omRunner.ResetAndConfigureCallCount()).To(Equal(1))
			productName, productVersion, configJSON := omRunner.ResetAndConfigureArgsForCall(0)
			Expect(productName).To(Equal("cf"))
			Expect(productVersion).To(Equal("1.2.3"))
			Expect(configJSON).To(MatchJSON(`{
					"product-properties": {
						".some-minimal-config":{
							"value":"some-value"
						},
						"some.additional.property": {
							"value": "some-value"
						}
					},
					"network-properties": {
						"network":{
							"name":"some-network"
						}
					}
				}`))

			Expect(omRunner.GetManifestCallCount()).To(Equal(1))
			productGUID := omRunner.GetManifestArgsForCall(0)
			Expect(productGUID).To(Equal("some-updated-guid"))
		})

		Describe("failure cases", func() {
			When("staged product cannot be found", func() {
				It("errors", func() {
					omRunner.FindStagedProductReturnsOnCall(0, internal.StagedProduct{}, errors.New("some error"))

					_, err := omService.RenderManifest(tileConfig, tileMetadata)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("some error"))
				})
			})

			When("manifest cannot be fetched", func() {
				It("errors", func() {
					omRunner.GetManifestReturns(nil, errors.New("some error"))

					_, err := omService.RenderManifest(tileConfig, tileMetadata)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("some error"))
				})
			})

			When("unable to reset and configure", func() {
				It("errors", func() {
					omRunner.ResetAndConfigureReturns(errors.New("reset and configure error"))

					_, err := omService.RenderManifest(tileConfig, tileMetadata)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("reset and configure error"))
				})
			})
		})
	})
})
