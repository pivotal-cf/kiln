package commands_test

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("Find the release version", func() {
	var (
		findReleaseVersion *commands.FindReleaseVersion
		logger             *log.Logger
		fakeReleasesSource *fakes.MultiReleaseSource

		writer strings.Builder

		fetchExecuteArgs []string
		executeErr       error
		releaseName      string
		someKilnfilePath string
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(&writer, "", 0)
			fakeReleasesSource = new(fakes.MultiReleaseSource)

			tmpDir, err := os.MkdirTemp("", "fetch-test")
			Expect(err).NotTo(HaveOccurred())
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
- name: has-constraint
  version: ~74.16.0
  source: bosh.io
- name: has-no-constraint
  source: bosh.io`

			someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
			err = os.WriteFile(someKilnfilePath, []byte(kilnContents), 0o644)
			Expect(err).NotTo(HaveOccurred())
			someKilnfileLockPath := filepath.Join(tmpDir, "Kilnfile.lock")
			err = os.WriteFile(someKilnfileLockPath, []byte(lockContents), 0o644)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			multiReleaseSourceProvider := func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) component.MultiReleaseSource {
				return fakeReleasesSource
			}
			findReleaseVersion = commands.NewFindReleaseVersion(logger, multiReleaseSourceProvider)

			logger.Printf("releaseName is: %s", releaseName)
			executeErr = findReleaseVersion.Execute(fetchExecuteArgs)
		})

		Context("when the release flag is missing", func() {
			It("returns an error", func() {
				err := findReleaseVersion.Execute([]string{})

				Expect(err).To(MatchError("missing required flag \"--release\""))
			})
		})

		When("there is no version constraint", func() {
			BeforeEach(func() {
				releaseName = "has-no-constraint"
				fetchExecuteArgs = []string{
					"--kilnfile", someKilnfilePath,
					"--release", releaseName,
				}
			})
			When("a latest release exists", func() {
				BeforeEach(func() {
					fakeReleasesSource.FindReleaseVersionReturns(component.Lock{
						Name: releaseName, Version: "74.12.5",
						RemotePath:   "remote_url",
						RemoteSource: "bosh.io",
						SHA1:         "some-sha",
					}, nil)
				})

				When("uaac has releases on bosh.io", func() {
					It("returns the latest release version", func() {
						Expect(executeErr).NotTo(HaveOccurred())
						args, _ := fakeReleasesSource.FindReleaseVersionArgsForCall(0)
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
			BeforeEach(func() {
				releaseName = "has-constraint"
				fetchExecuteArgs = []string{
					"--kilnfile", someKilnfilePath,
					"--release", releaseName,
				}
			})
			When("a release exists", func() {
				BeforeEach(func() {
					fakeReleasesSource.FindReleaseVersionReturns(component.Lock{
						Name: releaseName, Version: "74.16.5",
						RemotePath:   "remote_url",
						RemoteSource: "sourceId",
					}, nil)
				})

				When("uaa has releases on bosh.io", func() {
					It("returns the latest release version", func() {
						Expect(executeErr).NotTo(HaveOccurred())
						args, noDownload := fakeReleasesSource.FindReleaseVersionArgsForCall(0)
						Expect(noDownload).To(BeFalse())
						Expect(args.Version).To(Equal("~74.16.0"))
						Expect(args.StemcellVersion).To(Equal("4.5.6"))
						Expect(args.StemcellOS).To(Equal("some-os"))
						Expect((&writer).String()).To(ContainSubstring("\"74.16.5\""))
						Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"remote_url\""))
					})
				})
			})
		})

		When("--no-download is specified", func() {
			BeforeEach(func() {
				releaseName = "has-no-constraint"
				fetchExecuteArgs = []string{
					"--kilnfile", someKilnfilePath,
					"--release", releaseName,
					"--no-download",
				}
			})

			It("calls source with correct args", func() {
				Expect(executeErr).NotTo(HaveOccurred())
				_, noDownload := fakeReleasesSource.FindReleaseVersionArgsForCall(0)
				Expect(noDownload).To(BeTrue())
			})
		})

		When("--no-download is not specified", func() {
			BeforeEach(func() {
				releaseName = "has-no-constraint"
				fetchExecuteArgs = []string{
					"--kilnfile", someKilnfilePath,
					"--release", releaseName,
				}
			})

			It("calls source with correct args", func() {
				Expect(executeErr).NotTo(HaveOccurred())
				_, noDownload := fakeReleasesSource.FindReleaseVersionArgsForCall(0)
				Expect(noDownload).To(BeFalse())
			})
		})
	})
})
