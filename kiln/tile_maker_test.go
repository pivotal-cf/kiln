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
			FormDirectories:          []string{"some-forms-directory"},
			IconPath:                 "some-icon-path",
			InstanceGroupDirectories: []string{"some-instance-groups-directory"},
			JobDirectories:           []string{"some-jobs-directory"},
			Metadata:                 "some-metadata",
			MigrationDirectories:     []string{"some-migrations-directory"},
			OutputFile:               "some-output-dir/cool-product-file.1.2.3-build.4.pivotal",
			ReleaseDirectories:       []string{someReleasesDirectory, otherReleasesDirectory},
			RuntimeConfigDirectories: []string{"some-runtime-configs-directory"},
			StemcellTarball:          "some-stemcell-tarball",
			StubReleases:             true,
			VariableDirectories:      []string{"some-variables-directory"},
			Version:                  "1.2.3",
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

		buildInput := fakeMetadataBuilder.BuildArgsForCall(0)
		Expect(buildInput.MetadataPath).To(Equal("some-metadata"))
		Expect(buildInput.ReleaseTarballs).To(Equal(validReleases))
		Expect(buildInput.StemcellTarball).To(Equal("some-stemcell-tarball"))
		Expect(buildInput.FormDirectories).To(Equal([]string{"some-forms-directory"}))
		Expect(buildInput.InstanceGroupDirectories).To(Equal([]string{"some-instance-groups-directory"}))
		Expect(buildInput.JobDirectories).To(Equal([]string{"some-jobs-directory"}))
		Expect(buildInput.RuntimeConfigDirectories).To(Equal([]string{"some-runtime-configs-directory"}))
		Expect(buildInput.VariableDirectories).To(Equal([]string{"some-variables-directory"}))
		Expect(buildInput.IconPath).To(Equal("some-icon-path"))
		Expect(buildInput.Version).To(Equal("1.2.3"))
	})

	It("makes the tile", func() {
		fakeMetadataBuilder.BuildReturns(builder.GeneratedMetadata{
			IconImage: "some-icon-image",
			Name:      "cool-product-name",
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

		productName, generatedMetadataContents, actualConfig := fakeTileWriter.WriteArgsForCall(0)
		Expect(productName).To(Equal("cool-product-name"))
		Expect(generatedMetadataContents).To(MatchYAML(`
icon_image: some-icon-image
name: cool-product-name
releases:
- name: some-release
  file: some-release-tarball
  version: 1.2.3-build.4
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
  requires_cpi: false`))
		Expect(actualConfig).To(Equal(config))
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
