package commands_test

import (
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	release2 "github.com/pivotal-cf/kiln/pkg/release"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/fetcher"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
)

var _ = Describe("Find the release version", func() {
	var (
		findReleaseVersion commands.FindReleaseVersion
		logger             *log.Logger
		fakeReleasesSource *fetcherFakes.MultiReleaseSource

		writer strings.Builder

		fetchExecuteArgs []string
		executeErr       error
		releaseName      string
		someKilnfilePath string
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

			kilnContents := `
---
releases:
- name: uaa
  version: ~74.16.0
  source: bosh.io
- name: uaac
  source: bosh.io`

			someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
			err = ioutil.WriteFile(someKilnfilePath, []byte(kilnContents), 0644)
			Expect(err).NotTo(HaveOccurred())
			someKilnfileLockPath := filepath.Join(tmpDir, "Kilnfile.lock")
			err = ioutil.WriteFile(someKilnfileLockPath, []byte(lockContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			multiReleaseSourceProvider := func(kilnfile cargo2.Kilnfile, allowOnlyPublishable bool) fetcher.MultiReleaseSource {
				return fakeReleasesSource
			}
			findReleaseVersion = commands.NewFindReleaseVersion(logger, multiReleaseSourceProvider)

			fetchExecuteArgs = []string{
				"--kilnfile", someKilnfilePath,
				"--release", releaseName,
			}
			executeErr = findReleaseVersion.Execute(fetchExecuteArgs)
		})

		When("there is no version constraint", func() {
			When("a latest release exists", func() {
				BeforeEach(func() {
					releaseName = "uaac"
					fakeReleasesSource.FindReleaseVersionReturns(release2.Remote{
						ID:         release2.ID{Name: releaseName, Version: "74.12.5"},
						RemotePath: "remote_url",
						SourceID:   "bosh.io",
						SHA:        "some-sha",
					}, true, nil)
				})

				When("uaac has releases on bosh.io", func() {
					It("returns the latest release version", func() {
						Expect(executeErr).NotTo(HaveOccurred())
						args := fakeReleasesSource.FindReleaseVersionArgsForCall(0)
						Expect(args.StemcellVersion).To(Equal("4.5.6"))
						Expect(args.StemcellOS).To(Equal("some-os"))
						Expect(args.Version).To(Equal(""))
						Expect((&writer).String()).To(ContainSubstring("\"74.12.5\""))
						Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"remote_url\""))
						Expect((&writer).String()).To(ContainSubstring("\"source\":\"bosh.io\""))
						Expect((&writer).String()).To(ContainSubstring("\"sha\":\"some-sha\""))
					})
				})
			})
		})

		When("there is a version constraint", func() {
			When("a release exists", func() {
				BeforeEach(func() {
					releaseName = "uaa"
					fakeReleasesSource.FindReleaseVersionReturns(release2.Remote{
						ID:         release2.ID{Name: releaseName, Version: "74.16.5"},
						RemotePath: "remote_url",
						SourceID:   "sourceId",
					}, true, nil)
				})

				When("uaa has releases on bosh.io", func() {
					It("returns the latest release version", func() {
						Expect(executeErr).NotTo(HaveOccurred())
						args := fakeReleasesSource.FindReleaseVersionArgsForCall(0)
						Expect(args.VersionConstraint).To(Equal("~74.16.0"))
						Expect(args.StemcellVersion).To(Equal("4.5.6"))
						Expect(args.StemcellOS).To(Equal("some-os"))
						Expect((&writer).String()).To(ContainSubstring("\"74.16.5\""))
						Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"remote_url\""))
					})
				})
			})
		})
	})
})
