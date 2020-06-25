package commands_test

import (
	"github.com/pivotal-cf/kiln/commands"
	"log"
	"strings"

	fetcher "github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
)

var _ = Describe("Fetch", func() {
	var (
		mostRecentReleaseVersion commands.MostRecentReleaseVersion
		logger                   *log.Logger
		fakeBoshIOReleaseSource  *fetcherFakes.ReleaseSource
		fakeReleasesSource       fetcher.MultiReleaseSource

		writer strings.Builder

		fetchExecuteArgs []string
		executeErr       error
	)

	const (
		boshIOReleaseSourceID = fetcher.ReleaseSourceTypeBOSHIO
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(&writer, "", 0)

			fakeBoshIOReleaseSource = new(fetcherFakes.ReleaseSource)
			fakeBoshIOReleaseSource.IDReturns(boshIOReleaseSourceID)

		})

		JustBeforeEach(func() {
			fakeReleasesSource = fetcher.NewMultiReleaseSource(fakeBoshIOReleaseSource)

			mostRecentReleaseVersion = commands.NewMostRecentReleaseVersion(fakeReleasesSource, logger)

			executeErr = mostRecentReleaseVersion.Execute(fetchExecuteArgs)
		})

		When("a local compiled release exists", func() {
			var (
				releaseName string
			)
			BeforeEach(func() {
				releaseName = "uaa"
			})

			When("uaa has releases on bosh.io", func() {
				It("returns the latest release version", func() {
					Expect(executeErr).NotTo(HaveOccurred())
					Expect((&writer).String()).To(ContainSubstring("uaa: 74.12.5"))
				})
			})
		})
	})
})
