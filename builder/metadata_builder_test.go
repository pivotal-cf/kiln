package builder_test

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetadataBuilder", func() {
	var (
		releaseManifestReader  *fakes.ReleaseManifestReader
		stemcellManifestReader *fakes.StemcellManifestReader
		metadataReader         *fakes.MetadataReader
		logger                 *fakes.Logger
		tileBuilder            builder.MetadataBuilder
	)

	BeforeEach(func() {
		releaseManifestReader = &fakes.ReleaseManifestReader{}
		stemcellManifestReader = &fakes.StemcellManifestReader{}
		metadataReader = &fakes.MetadataReader{}
		logger = &fakes.Logger{}

		releaseManifestReader.ReadCall.Stub = func(path string) (builder.ReleaseManifest, error) {
			switch path {
			case "/path/to/release-1.tgz":
				return builder.ReleaseManifest{
					Name:    "release-1",
					Version: "version-1",
				}, nil
			case "/path/to/release-2.tgz":
				return builder.ReleaseManifest{
					Name:    "release-2",
					Version: "version-2",
				}, nil
			default:
				return builder.ReleaseManifest{}, fmt.Errorf("could not read release %q", path)
			}
		}
		stemcellManifestReader.ReadReturns(builder.StemcellManifest{
			Version:         "2332",
			OperatingSystem: "ubuntu-trusty",
		},
			nil,
		)

		tileBuilder = builder.NewMetadataBuilder(releaseManifestReader, stemcellManifestReader, metadataReader, logger)
	})

	Describe("Build", func() {
		It("creates a GeneratedMetadata with the correct information", func() {
			metadataReader.ReadReturns(builder.Metadata{
				"name":                      "cool-product",
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
			},
				nil,
			)
			generatedMetadata, err := tileBuilder.Build([]string{
				"/path/to/release-1.tgz",
				"/path/to/release-2.tgz",
			}, "/path/to/test-stemcell.tgz", "/some/path/metadata.yml", "1.2.3", "/path/to/tile.zip")
			Expect(err).NotTo(HaveOccurred())
			Expect(stemcellManifestReader.ReadArgsForCall(0)).To(Equal("/path/to/test-stemcell.tgz"))
			metadataPath, version := metadataReader.ReadArgsForCall(0)
			Expect(metadataPath).To(Equal("/some/path/metadata.yml"))
			Expect(version).To(Equal("1.2.3"))

			Expect(generatedMetadata.Name).To(Equal("cool-product"))
			Expect(generatedMetadata.Releases).To(Equal([]builder.Release{
				{
					Name:    "release-1",
					Version: "version-1",
					File:    "release-1.tgz",
				},
				{
					Name:    "release-2",
					Version: "version-2",
					File:    "release-2.tgz",
				},
			}))
			Expect(generatedMetadata.StemcellCriteria).To(Equal(builder.StemcellCriteria{
				Version:     "2332",
				OS:          "ubuntu-trusty",
				RequiresCPI: false,
			}))
			Expect(generatedMetadata.Metadata).To(Equal(builder.Metadata{
				"metadata_version":          "some-metadata-version",
				"provides_product_versions": "some-provides-product-versions",
			}))

			Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
				"Creating metadata for /path/to/tile.zip...",
				"Read manifest for release release-1",
				"Read manifest for release release-2",
				"Read manifest for stemcell version 2332",
				"Read metadata",
			}))
		})

		Context("failure cases", func() {
			Context("when the release tarball cannot be read", func() {
				It("returns an error", func() {
					releaseManifestReader.ReadCall.Stub = nil
					releaseManifestReader.ReadCall.Returns.Error = errors.New("failed to read release tarball")

					_, err := tileBuilder.Build([]string{"release-1.tgz"}, "", "", "", "")
					Expect(err).To(MatchError("failed to read release tarball"))
				})
			})

			Context("when the stemcell tarball cannot be read", func() {
				It("returns an error", func() {
					stemcellManifestReader.ReadReturns(builder.StemcellManifest{}, errors.New("failed to read stemcell tarball"))

					_, err := tileBuilder.Build([]string{}, "stemcell.tgz", "", "", "")
					Expect(err).To(MatchError("failed to read stemcell tarball"))
				})
			})

			Context("when the metadata cannot be read", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{}, errors.New("failed to read metadata"))

					_, err := tileBuilder.Build([]string{}, "", "metadata.yml", "", "")
					Expect(err).To(MatchError("failed to read metadata"))
				})
			})

			Context("when the metadata does not contain a product name", func() {
				It("returns an error", func() {
					metadataReader.ReadReturns(builder.Metadata{
						"metadata_version":          "some-metadata-version",
						"provides_product_versions": "some-provides-product-versions",
					},
						nil,
					)

					_, err := tileBuilder.Build([]string{}, "", "metadata.yml", "", "")
					Expect(err).To(MatchError(`missing "name" in tile metadata`))
				})
			})
		})
	})
})
