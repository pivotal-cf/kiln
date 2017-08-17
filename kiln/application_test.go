package kiln_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/kiln"
	"github.com/pivotal-cf/kiln/kiln/fakes"
)

var _ = Describe("application", func() {
	var (
		argParser *fakes.ArgParser
		tileMaker *fakes.TileMaker
		app       kiln.Application
	)

	BeforeEach(func() {
		argParser = &fakes.ArgParser{}
		tileMaker = &fakes.TileMaker{}
		app = kiln.NewApplication(argParser, tileMaker)
	})

	Describe("Run", func() {
		It("parses args", func() {
			err := app.Run([]string{
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

			err := app.Run([]string{})

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

	Context("failure cases", func() {
		Context("when the args cannot be parsed", func() {
			It("returns an error", func() {
				argParser.ParseReturns(kiln.ApplicationConfig{}, errors.New("a parse error occurred"))

				err := app.Run([]string{"does not matter"})
				Expect(err).To(MatchError("a parse error occurred"))
			})
		})

		Context("when the maker fails", func() {
			It("returns an error", func() {
				tileMaker.MakeReturns(errors.New("a maker error occurred"))

				err := app.Run([]string{"does not matter"})
				Expect(err).To(MatchError("a maker error occurred"))
			})
		})
	})
})
