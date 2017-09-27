package commands_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/kiln"
	"github.com/pivotal-cf/kiln/kiln/fakes"
)

var _ = Describe("bake", func() {
	var (
		argParser *fakes.ArgParser
		tileMaker *fakes.TileMaker
		bake      commands.Bake
	)

	BeforeEach(func() {
		argParser = &fakes.ArgParser{}
		tileMaker = &fakes.TileMaker{}
		bake = commands.NewBake(argParser, tileMaker)
	})

	Describe("Execute", func() {
		It("parses args", func() {
			err := bake.Execute([]string{
				"foo", "bar",
				"gaz", "goo",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(argParser.ParseCallCount()).To(Equal(1))
			Expect(argParser.ParseArgsForCall(0)).To(Equal([]string{
				"foo", "bar",
				"gaz", "goo",
			}))
		})

		It("builds the tile", func() {
			argParser.ParseReturns(kiln.ApplicationConfig{
				StemcellTarball: "some-stemcell-tarball",
				ReleaseTarballs: []string{"some-release-tarball", "some-other-release-tarball"},
				Handcraft:       "some-handcraft",
				Version:         "1.2.3-build.4",
				FinalVersion:    "1.2.3",
				ProductName:     "cool-product-name",
				FilenamePrefix:  "cool-product-file",
				OutputDir:       "some-output-dir",
			}, nil)

			err := bake.Execute([]string{})

			Expect(err).NotTo(HaveOccurred())
			Expect(tileMaker.MakeCallCount()).To(Equal(1))

			config := tileMaker.MakeArgsForCall(0)
			Expect(config).To(Equal(kiln.ApplicationConfig{
				StemcellTarball: "some-stemcell-tarball",
				ReleaseTarballs: []string{"some-release-tarball", "some-other-release-tarball"},
				Handcraft:       "some-handcraft",
				Version:         "1.2.3-build.4",
				FinalVersion:    "1.2.3",
				ProductName:     "cool-product-name",
				FilenamePrefix:  "cool-product-file",
				OutputDir:       "some-output-dir",
			}))
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			command := commands.NewBake(nil, nil)
			Expect(command.Usage()).To(Equal(commands.Usage{
				Description:      "Builds a tile to be uploaded to OpsMan from provided inputs.",
				ShortDescription: "builds a tile",
				Flags:            command.Options,
			}))
		})
	})
})
