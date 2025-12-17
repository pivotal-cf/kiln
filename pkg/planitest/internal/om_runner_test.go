package internal_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/planitest/internal"
	"github.com/pivotal-cf/kiln/pkg/planitest/internal/fakes"
)

var _ = Describe("OMRunner", func() {
	var (
		cmdRunner *fakes.CommandRunner
		omRunner  internal.OMRunner
	)

	BeforeEach(func() {
		cmdRunner = &fakes.CommandRunner{}
		omRunner = internal.NewOMRunner(cmdRunner, internal.RealIO)
	})

	Describe("StagedProducts", func() {
		It("executes an om curl to retrieve the list of staged products", func() {
			stagedResponse := `[{
  "guid": "some-guid",
	"type": "cf",
	"product_version": "1.1.1"
}]`
			cmdRunner.RunReturns(stagedResponse, "", nil)

			stagedProducts, err := omRunner.StagedProducts()
			Expect(err).NotTo(HaveOccurred())

			Expect(cmdRunner.RunCallCount()).To(Equal(1))
			command, args := cmdRunner.RunArgsForCall(0)
			Expect(command).To(Equal("om"))
			Expect(args).To(Equal([]string{
				"--skip-ssl-validation",
				"curl",
				"--path", "/api/v0/staged/products",
			}))

			Expect(stagedProducts).To(Equal([]internal.StagedProduct{
				{
					GUID:           "some-guid",
					Type:           "cf",
					ProductVersion: "1.1.1",
				},
			}))
		})

		When("the set of staged products cannot be retrieved", func() {
			It("errors", func() {
				cmdRunner.RunReturns("", "stderr output", errors.New("some error"))

				_, err := omRunner.StagedProducts()
				Expect(err).To(MatchError("unable to retrieve staged products: some error: stderr output"))
			})
		})

		When("the staged products response is not well-formed JSON", func() {
			It("errors", func() {
				cmdRunner.RunReturns("not-well-formed-json", "", nil)

				_, err := omRunner.StagedProducts()
				Expect(err).To(MatchError(HavePrefix("unable to retrieve staged products")))
			})
		})
	})

	Describe("FindStagedProduct", func() {
		It("returns staged product", func() {
			stagedResponse := `[{
  "guid": "some-guid",
	"type": "cf",
	"product_version": "1.1.1"
}]`
			cmdRunner.RunReturns(stagedResponse, "", nil)

			stagedProduct, err := omRunner.FindStagedProduct("cf")
			Expect(err).NotTo(HaveOccurred())

			Expect(stagedProduct).To(Equal(internal.StagedProduct{
				GUID:           "some-guid",
				Type:           "cf",
				ProductVersion: "1.1.1",
			}))
		})

		When("the specified product has not been staged", func() {
			BeforeEach(func() {
				stagedResponse := `[{
  "guid": "some-guid",
	"type": "not-cf",
	"product_version": "1.1.1"
},{
  "guid": "some-other-guid",
	"type": "also-not-cf",
	"product_version": "2.2.2"
}]`
				cmdRunner.RunReturns(stagedResponse, "", nil)
			})

			It("returns an error with current staged products", func() {
				_, err := omRunner.FindStagedProduct("cf")
				Expect(err).To(MatchError(`product "cf" has not been staged. Staged products: "not-cf, also-not-cf"`))
			})
		})
	})

	Describe("ResetAndConfigure", func() {
		var (
			configFile         *os.File
			omRunnerWithFileIO internal.OMRunner
			fileIO             *fakes.FileIO
			err                error
		)

		BeforeEach(func() {
			fileIO = &fakes.FileIO{}
			omRunnerWithFileIO = internal.NewOMRunner(cmdRunner, fileIO)

			productPropertiesFile := `---
product-properties:
  .some-minimal-config":
    value: some-value
`
			networkPropertiesFile := `---
network-properties:
  network:
	  name: some-network
`
			configFile, err = os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			fileIO.TempFileReturns(configFile, nil)

			_, err = configFile.WriteString(productPropertiesFile)
			Expect(err).NotTo(HaveOccurred())

			_, err = configFile.WriteString(networkPropertiesFile)
			Expect(err).NotTo(HaveOccurred())
		})

		When("providing a config file with product-properties and network-properties", func() {
			It("reverts staged changes, stages product, and configures network + properties", func() {
				configYML := `---
product-properties:
  .some-minimal-config":
    value: some-value	
network-properties:
  network:
	  name: some-network
`

				err := omRunnerWithFileIO.ResetAndConfigure("cf", "1.2.3", configYML)
				Expect(err).NotTo(HaveOccurred())

				Expect(fileIO.TempFileCallCount()).To(Equal(1))

				Expect(fileIO.RemoveCallCount()).To(Equal(1))
				name := fileIO.RemoveArgsForCall(0)
				Expect(name).To(Equal(configFile.Name()))

				Expect(cmdRunner.RunCallCount()).To(Equal(3))
				command, args := cmdRunner.RunArgsForCall(0)
				Expect(command).To(Equal("om"))
				Expect(args).To(Equal([]string{
					"--skip-ssl-validation",
					"curl",
					"-x", "DELETE",
					"--path", "/api/v0/staged",
				}))

				command, args = cmdRunner.RunArgsForCall(1)
				Expect(command).To(Equal("om"))
				Expect(args).To(Equal([]string{
					"--skip-ssl-validation",
					"stage-product",
					"--product-name", "cf",
					"--product-version", "1.2.3",
				}))

				command, args = cmdRunner.RunArgsForCall(2)
				Expect(command).To(Equal("om"))
				Expect(args).To(Equal([]string{
					"--skip-ssl-validation",
					"configure-product",
					"--config", configFile.Name(),
				}))
			})
		})

		Describe("failure cases", func() {
			When("the request to revert staged changes fails", func() {
				It("errors", func() {
					cmdRunner.RunReturnsOnCall(0, "", "stderr output", errors.New("some error"))

					err := omRunnerWithFileIO.ResetAndConfigure("cf", "1.2.3", "{}")
					Expect(err).To(MatchError(`unable to revert staged changes: some error: stderr output`))
				})
			})

			When("the request to stage the product fails", func() {
				It("errors", func() {
					cmdRunner.RunReturnsOnCall(1, "", "stderr output", errors.New("some error"))

					err := omRunnerWithFileIO.ResetAndConfigure("cf", "1.2.3", "{}")
					Expect(err).To(MatchError(`unable to stage product "cf", version "1.2.3": some error: stderr output`))
				})
			})

			When("the request to configure the product fails", func() {
				It("errors", func() {
					cmdRunner.RunReturnsOnCall(2, "", "stderr output", errors.New("some error"))

					err := omRunnerWithFileIO.ResetAndConfigure("cf", "1.2.3", "{}")
					Expect(err).To(MatchError(`unable to configure product "cf": some error: stderr output`))
				})
			})

			When("tempfile cannot be recreated", func() {
				It("errors", func() {
					fileIO.TempFileReturns(nil, errors.New("some error"))

					err := omRunnerWithFileIO.ResetAndConfigure("cf", "1.2.3", "{}")
					Expect(err).To(MatchError(ContainSubstring(`some error`)))
				})
			})
		})
	})

	Describe("GetManifest", func() {
		It("executes an om curl to retrieve the manifest", func() {
			cmdRunner.RunReturns(`{
  "manifest": {
    "name": "cf-some-guid",
    "releases": [
      {
        "name": "some-release",
        "version": "1.2.3"
      }
    ]
  }
}`, "", nil)
			manifest, err := omRunner.GetManifest("some-guid")
			Expect(err).ToNot(HaveOccurred())

			Expect(cmdRunner.RunCallCount()).To(Equal(1))
			command, args := cmdRunner.RunArgsForCall(0)
			Expect(command).To(Equal("om"))
			Expect(args).To(Equal([]string{
				"--skip-ssl-validation",
				"curl",
				"--path", "/api/v0/staged/products/some-guid/manifest",
			}))

			Expect(manifest).To(Equal(map[string]any{
				"name": "cf-some-guid",
				"releases": []any{
					map[string]any{
						"name":    "some-release",
						"version": "1.2.3",
					},
				},
			}))
		})

		Context("failure cases", func() {
			When("the staged manifest cannot be retrieved", func() {
				It("errors", func() {
					cmdRunner.RunReturns("", "stderr output", errors.New("some error"))

					_, err := omRunner.GetManifest("some-guid")
					Expect(err).To(MatchError(`unable to retrieve staged manifest for product guid "some-guid": some error: stderr output`))
				})
			})

			When("the staged manifest response is not well-formed JSON", func() {
				It("errors", func() {
					cmdRunner.RunReturns("not-well-formed-json", "", nil)

					_, err := omRunner.GetManifest("some-guid")
					Expect(err).To(MatchError(HavePrefix(`unable to retrieve staged manifest for product guid "some-guid"`)))
				})
			})

			When("the product config is incomplete", func() {
				It("errors", func() {
					cmdRunner.RunReturns(`{"errors": {"base": ["Product configuration is incomplete; cannot generate manifest"]}}`, "", nil)

					_, err := omRunner.GetManifest("some-guid")
					Expect(err).To(MatchError(`unable to retrieve staged manifest for product guid "some-guid": Product configuration is incomplete; cannot generate manifest`))
				})
			})
		})
	})
})
