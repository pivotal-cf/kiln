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
		releases               []string
		err                    error
	)

	BeforeEach(func() {
		someReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		someReleaseFile, err := ioutil.TempFile(someReleasesDirectory, "")
		Expect(err).NotTo(HaveOccurred())

		otherReleaseFile, err := ioutil.TempFile(otherReleasesDirectory, "")
		Expect(err).NotTo(HaveOccurred())

		releases = []string{someReleaseFile.Name(), otherReleaseFile.Name()}

		fakeMetadataBuilder = &fakes.MetadataBuilder{}
		fakeTileWriter = &fakes.TileWriter{}
		fakeLogger = &fakes.Logger{}

		config = commands.BakeConfig{
			ProductName:          "cool-product-name",
			Version:              "1.2.3",
			StemcellTarball:      "some-stemcell-tarball",
			ReleaseDirectories:   []string{someReleasesDirectory, otherReleasesDirectory},
			Handcraft:            "some-handcraft",
			MigrationDirectories: []string{"some-migrations-directory"},
			BaseContentMigration: "some-base-content-migration",
			ContentMigrations:    []string{"some-content-migration", "some-other-content-migration"},
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

		releaseTarballs, stemcellTarball, handcraft, name, version, outputPath := fakeMetadataBuilder.BuildArgsForCall(0)
		Expect(releaseTarballs).To(Equal(releases))
		Expect(stemcellTarball).To(Equal("some-stemcell-tarball"))
		Expect(handcraft).To(Equal("some-handcraft"))
		Expect(name).To(Equal("cool-product-name"))
		Expect(version).To(Equal("1.2.3"))
		Expect(outputPath).To(Equal("some-output-dir/cool-product-file.1.2.3-build.4.pivotal"))
	})

	It("makes the tile", func() {
		fakeMetadataBuilder.BuildReturns(builder.Metadata{
			Name: "cool-product-name",
			Releases: []builder.MetadataRelease{{
				Name:    "some-release",
				File:    "some-release-tarball",
				Version: "1.2.3-build.4",
			}},
			StemcellCriteria: builder.MetadataStemcellCriteria{
				Version:     "2.3.4",
				OS:          "an-operating-system",
				RequiresCPI: false,
			},
		}, nil)

		err := tileMaker.Make(config)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeTileWriter.WriteCallCount()).To(Equal(1))

		metadataContents, config := fakeTileWriter.WriteArgsForCall(0)
		Expect(metadataContents).To(MatchYAML(`
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
			ProductName:          "cool-product-name",
			Version:              "1.2.3",
			StemcellTarball:      "some-stemcell-tarball",
			ReleaseDirectories:   []string{someReleasesDirectory, otherReleasesDirectory},
			Handcraft:            "some-handcraft",
			MigrationDirectories: []string{"some-migrations-directory"},
			BaseContentMigration: "some-base-content-migration",
			ContentMigrations:    []string{"some-content-migration", "some-other-content-migration"},
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
		Context("when metadata builder fails", func() {
			It("returns an error", func() {
				fakeMetadataBuilder.BuildReturns(builder.Metadata{}, errors.New("some-error"))

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
