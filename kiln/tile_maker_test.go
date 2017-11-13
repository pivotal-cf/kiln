package kiln_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/kiln"
	"github.com/pivotal-cf/kiln/kiln/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TileMaker", func() {
	var (
		fakeMetadataBuilder *fakes.MetadataBuilder
		fakeTileWriter      *fakes.TileWriter
		fakeLogger          *fakes.Logger

		config                 commands.BakeConfig
		tileMaker              kiln.TileMaker
		someReleasesDirectory  string
		otherReleasesDirectory string
		validReleases          []string
		err                    error
	)

	BeforeEach(func() {
		someReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		tarballRelease := someReleasesDirectory + "/release1.tgz"
		err = ioutil.WriteFile(tarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		otherTarballRelease := someReleasesDirectory + "/release2.tgz"
		err = ioutil.WriteFile(otherTarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		nonTarballRelease := someReleasesDirectory + "/some-broken-release"
		err = ioutil.WriteFile(nonTarballRelease, []byte(""), 0644)
		Expect(err).NotTo(HaveOccurred())

		validReleases = []string{tarballRelease, otherTarballRelease}

		fakeMetadataBuilder = &fakes.MetadataBuilder{}
		fakeTileWriter = &fakes.TileWriter{}
		fakeLogger = &fakes.Logger{}

		config = commands.BakeConfig{
			Version:              "1.2.3",
			StemcellTarball:      "some-stemcell-tarball",
			ReleaseDirectories:   []string{someReleasesDirectory, otherReleasesDirectory},
			Metadata:             "some-metadata",
			MigrationDirectories: []string{"some-migrations-directory"},
			OutputFile:           "some-output-dir/cool-product-file.1.2.3-build.4.pivotal",
			StubReleases:         true,
		}
		tileMaker = kiln.NewTileMaker(fakeMetadataBuilder, fakeTileWriter, fakeLogger)
	})

	AfterEach(func() {
		os.Remove(someReleasesDirectory)
		os.Remove(otherReleasesDirectory)
	})

	It("builds the metadata", func() {
		err := tileMaker.Make(config)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeMetadataBuilder.BuildCallCount()).To(Equal(1))

		releaseTarballs, stemcellTarball, metadata, version, outputPath := fakeMetadataBuilder.BuildArgsForCall(0)
		Expect(releaseTarballs).To(Equal(validReleases))
		Expect(stemcellTarball).To(Equal("some-stemcell-tarball"))
		Expect(metadata).To(Equal("some-metadata"))
		Expect(version).To(Equal("1.2.3"))
		Expect(outputPath).To(Equal("some-output-dir/cool-product-file.1.2.3-build.4.pivotal"))
	})

	It("makes the tile", func() {
		fakeMetadataBuilder.BuildReturns(builder.GeneratedMetadata{
			Name: "cool-product-name",
			Releases: []builder.Release{{
				Name:    "some-release",
				File:    "some-release-tarball",
				Version: "1.2.3-build.4",
			}},
			StemcellCriteria: builder.StemcellCriteria{
				Version:     "2.3.4",
				OS:          "an-operating-system",
				RequiresCPI: false,
			},
		}, nil)

		err := tileMaker.Make(config)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))

		productName, generatedMetadataContents, config := fakeTileWriter.WriteArgsForCall(0)
		Expect(productName).To(Equal("cool-product-name"))
		Expect(generatedMetadataContents).To(MatchYAML(`
name: cool-product-name
releases:
- name: some-release
  file: some-release-tarball
  version: 1.2.3-build.4
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
  requires_cpi: false`))
		Expect(config).To(Equal(commands.BakeConfig{
			Version:              "1.2.3",
			StemcellTarball:      "some-stemcell-tarball",
			ReleaseDirectories:   []string{someReleasesDirectory, otherReleasesDirectory},
			Metadata:             "some-metadata",
			MigrationDirectories: []string{"some-migrations-directory"},
			OutputFile:           "some-output-dir/cool-product-file.1.2.3-build.4.pivotal",
			StubReleases:         true,
		}))
	})

	It("logs its step", func() {
		err := tileMaker.Make(config)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLogger.PrintlnCall.Receives.LogLines).To(Equal([]string{"Marshaling metadata file..."}))
	})

	Context("failure cases", func() {
		Context("when generated metadata builder fails", func() {
			It("returns an error", func() {
				fakeMetadataBuilder.BuildReturns(builder.GeneratedMetadata{}, errors.New("some-error"))

				err := tileMaker.Make(config)
				Expect(err).To(MatchError("some-error"))
			})
		})

		Context("when the tile writer fails", func() {
			It("returns an error", func() {
				fakeTileWriter.WriteReturns(errors.New("tile writer has failed"))

				err := tileMaker.Make(config)
				Expect(err).To(MatchError("tile writer has failed"))
			})
		})
	})
})
