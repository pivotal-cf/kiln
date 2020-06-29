package commands_test

import (
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
)

var _ = Describe("Fetch the most recent release version", func() {
	var (
		mostRecentReleaseVersion commands.MostRecentReleaseVersion
		logger                   *log.Logger
		fakeReleasesSource       *fetcherFakes.MultiReleaseSource

		writer strings.Builder

		fetchExecuteArgs []string
		executeErr       error
		releaseName = "uaa"

	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(&writer, "", 0)
			fakeReleasesSource = new(fetcherFakes.MultiReleaseSource)

			tmpDir, err := ioutil.TempDir("", "fetch-test")
			lockContents := `
---
releases:
- name: some-release
  version: "1.2.3"
  remote_source:
  remote_path: my-remote-path
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`


			someKilnfilePath := filepath.Join(tmpDir, "Kilnfile")
			err = ioutil.WriteFile(someKilnfilePath, []byte(""), 0644)
			Expect(err).NotTo(HaveOccurred())
			someKilnfileLockPath := filepath.Join(tmpDir, "Kilnfile.lock")
			err = ioutil.WriteFile(someKilnfileLockPath, []byte(lockContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			fetchExecuteArgs = []string{
				"--kilnfile", someKilnfilePath,
				"--release", releaseName,
			}

		})

		JustBeforeEach(func() {
			multiReleaseSourceProvider := func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) fetcher.MultiReleaseSource {
				return fakeReleasesSource
			}
			mostRecentReleaseVersion = commands.NewMostRecentReleaseVersion(logger, multiReleaseSourceProvider)

			executeErr = mostRecentReleaseVersion.Execute(fetchExecuteArgs)
		})

		When("a latest release exists", func() {
			var (
				releaseName string
			)
			BeforeEach(func() {
				releaseName = "uaa"
				fakeReleasesSource.GetLatestReleaseVersionReturns(release.Remote{
					ID: release.ID{Name: releaseName, Version: "74.12.5"},
					RemotePath: "remote_url",
					SourceID: "sourceId",
				}, true, nil)
			})

			When("uaa has releases on bosh.io", func() {
				It("returns the latest release version", func() {
					Expect(executeErr).NotTo(HaveOccurred())
					Expect((&writer).String()).To(ContainSubstring("\"74.12.5\""))
					Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"remote_url\""))
				})
			})
		})
	})
})
