package planitest

import (
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/planitest/internal/fakes"
)

var _ = Describe("Product Service", func() {
	var config ProductConfig

	BeforeEach(func() {
		config = ProductConfig{
			ConfigFile: strings.NewReader("/some/config/file"),
			TileFile:   strings.NewReader("/some/tile/file"),
		}
	})

	Describe("RENDERERS", func() {
		var (
			err                         error
			rendererEnvironmentVariable string
		)

		JustBeforeEach(func() {
			err = os.Setenv("RENDERER", rendererEnvironmentVariable)
			Expect(err).NotTo(HaveOccurred())

			_, err = NewProductService(config)
		})

		When("set to om", func() {
			BeforeEach(func() {
				rendererEnvironmentVariable = "om"
			})

			It("does not error", func() { Expect(err).NotTo(HaveOccurred()) })
		})
		When("set to ops-manifest", func() {
			BeforeEach(func() {
				rendererEnvironmentVariable = "ops-manifest"
			})

			It("does not error", func() { Expect(err).NotTo(HaveOccurred()) })
		})
		When("set to neither", func() {
			BeforeEach(func() {
				rendererEnvironmentVariable = ""
			})

			It("errors", func() { Expect(err).To(MatchError("RENDERER must be set to om or ops-manifest")) })
		})
	})

	Describe("NewProductService", func() {
		Context("failure cases", func() {
			When("the config file is not specified", func() {
				It("errors", func() {
					config.ConfigFile = nil
					_, err := NewProductService(config)
					Expect(err).To(MatchError("Config file must be provided"))
				})
			})

			When("the metadata file is not specified", func() {
				It("errors", func() {
					config.TileFile = nil
					_, err := NewProductService(config)
					Expect(err).To(MatchError("Tile file must be provided"))
				})
			})
		})
	})

	Describe("RenderManifest", func() {
		var (
			productService *ProductService
			renderService  *fakes.RenderService
			productConfig  ProductConfig
		)

		BeforeEach(func() {
			renderService = new(fakes.RenderService)
		})

		JustBeforeEach(func() {
			productService = &ProductService{config: productConfig, renderService: renderService}
		})

		When("there are no product properties", func() {
			BeforeEach(func() {
				productConfig = ProductConfig{
					ConfigFile: strings.NewReader("product-properties: {}\nnetwork-properties: {key: value}"),
					TileFile:   strings.NewReader("tile-yaml-goes-here"),
				}
			})

			It("sets the key's value to an empty hash in the YAML", func() {
				_, err := productService.RenderManifest(nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(renderService.RenderManifestCallCount()).To(Equal(1))
				tileConfigReader, _ := renderService.RenderManifestArgsForCall(0)

				tileConfig, err := io.ReadAll(tileConfigReader)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(tileConfig)).To(ContainSubstring("product-properties: {}"))
			})
		})
	})
})
