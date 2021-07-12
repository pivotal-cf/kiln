package commands_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/fetcher"
	fetcherFakes "github.com/pivotal-cf/kiln/internal/fetcher/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/release"
)

var _ = Describe("Fetch", func() {
	var (
		fetch                       commands.Fetch
		logger                      *log.Logger
		tmpDir                      string
		someKilnfilePath            string
		someKilnfileLockPath        string
		lockContents                string
		someReleasesDirectory       string
		fakeS3CompiledReleaseSource *fetcherFakes.ReleaseSource
		fakeBoshIOReleaseSource     *fetcherFakes.ReleaseSource
		fakeS3BuiltReleaseSource    *fetcherFakes.ReleaseSource
		fakeReleaseSources          fetcher.MultiReleaseSource
		fakeLocalReleaseDirectory   *commandsFakes.LocalReleaseDirectory
		multiReleaseSourceProvider  commands.MultiReleaseSourceProvider

		fetchExecuteArgs []string
		fetchExecuteErr  error
	)

	const (
		s3CompiledReleaseSourceID = "s3-compiled"
		s3BuiltReleaseSourceID    = "s3-built"
		boshIOReleaseSourceID     = fetcher.ReleaseSourceTypeBOSHIO
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(GinkgoWriter, "", 0)

			var err error
			tmpDir, err = ioutil.TempDir("", "fetch-test")

			someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
			Expect(err).NotTo(HaveOccurred())

			someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
			err = ioutil.WriteFile(someKilnfilePath, []byte(""), 0644)
			Expect(err).NotTo(HaveOccurred())

			someKilnfileLockPath = filepath.Join(tmpDir, "Kilnfile.lock")
			lockContents = `
---
releases:
- name: some-release
  version: "1.2.3"
  remote_source: ` + s3CompiledReleaseSourceID + `
  remote_path: my-remote-path
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`

			fakeLocalReleaseDirectory = new(commandsFakes.LocalReleaseDirectory)

			fakeS3CompiledReleaseSource = new(fetcherFakes.ReleaseSource)
			fakeS3CompiledReleaseSource.IDReturns(s3CompiledReleaseSourceID)
			fakeBoshIOReleaseSource = new(fetcherFakes.ReleaseSource)
			fakeBoshIOReleaseSource.IDReturns(boshIOReleaseSourceID)
			fakeS3BuiltReleaseSource = new(fetcherFakes.ReleaseSource)
			fakeS3BuiltReleaseSource.IDReturns(s3BuiltReleaseSourceID)

			fetchExecuteArgs = []string{
				"--releases-directory", someReleasesDirectory,
				"--kilnfile", someKilnfilePath,
			}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		JustBeforeEach(func() {
			fakeReleaseSources = fetcher.NewMultiReleaseSource(fakeS3CompiledReleaseSource, fakeBoshIOReleaseSource, fakeS3BuiltReleaseSource)
			multiReleaseSourceProvider = func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) fetcher.MultiReleaseSource {
				return fakeReleaseSources
			}

			err := ioutil.WriteFile(someKilnfileLockPath, []byte(lockContents), 0644)
			Expect(err).NotTo(HaveOccurred())
			fetch = commands.NewFetch(logger, multiReleaseSourceProvider, fakeLocalReleaseDirectory)

			fetchExecuteErr = fetch.Execute(fetchExecuteArgs)
		})

		When("a local compiled release exists", func() {
			const (
				expectedStemcellOS      = "fooOS"
				expectedStemcellVersion = "0.2.0"
			)
			var (
				releaseID     release.ID
				releaseOnDisk release.Local
			)
			BeforeEach(func() {
				releaseID = release.ID{Name: "some-release", Version: "0.1.0"}
				fakeS3CompiledReleaseSource.DownloadReleaseReturns(
					release.Local{
						ID:        releaseID,
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
						SHA1:      "correct-sha",
					}, nil)
				lockContents = `---
releases:
- name: ` + releaseID.Name + `
  version: "` + releaseID.Version + `"
  remote_source: ` + s3CompiledReleaseSourceID + `
  remote_path: not-used
  sha1: correct-sha
stemcell_criteria:
  os: ` + expectedStemcellOS + `
  version: "` + expectedStemcellVersion + `"`
				fetchExecuteArgs = append(fetchExecuteArgs, "--no-confirm")
			})

			When("the release on disk has the wrong SHA1", func() {
				BeforeEach(func() {
					releaseOnDisk = release.Local{
						ID:        releaseID,
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
						SHA1:      "wrong-sha",
					}
					fakeLocalReleaseDirectory.GetLocalReleasesReturns([]release.Local{releaseOnDisk}, nil)
				})

				It("deletes the file from disk", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveLen(1))
					Expect(extras).To(ConsistOf(releaseOnDisk))
				})
			})

			When("the release on disk has the correct SHA1", func() {
				BeforeEach(func() {
					releaseOnDisk = release.Local{
						ID:        releaseID,
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
						SHA1:      "correct-sha",
					}
					fakeLocalReleaseDirectory.GetLocalReleasesReturns([]release.Local{releaseOnDisk}, nil)
				})

				It("does not delete the file from disk", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(0))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveLen(0))
				})
			})
		})

		Context("starting with no releases but all can be downloaded from their source (happy path)", func() {
			var (
				s3CompiledReleaseID = release.ID{Name: "lts-compiled-release", Version: "1.2.4"}
				s3BuiltReleaseID    = release.ID{Name: "lts-built-release", Version: "1.3.9"}
				boshIOReleaseID     = release.ID{Name: "boshio-release", Version: "1.4.16"}
			)
			BeforeEach(func() {
				lockContents = `---
releases:
- name: lts-compiled-release
  version: "1.2.4"
  remote_source: ` + s3CompiledReleaseSourceID + `
  remote_path: some-s3-key
  sha1: correct-sha
- name: lts-built-release
  version: "1.3.9"
  remote_source: ` + s3BuiltReleaseSourceID + `
  remote_path: some-other-s3-key
  sha1: correct-sha
- name: boshio-release
  version: "1.4.16"
  remote_source: ` + boshIOReleaseSourceID + `
  remote_path: some-bosh-io-url
  sha1: correct-sha
stemcell_criteria:
  os: some-os
  version: "30.1"
`
				fakeS3CompiledReleaseSource.DownloadReleaseReturns(
					release.Local{ID: s3CompiledReleaseID, LocalPath: "local-path", SHA1: "correct-sha"},
					nil)

				fakeS3BuiltReleaseSource.DownloadReleaseReturns(
					release.Local{ID: s3BuiltReleaseID, LocalPath: "local-path2", SHA1: "correct-sha"},
					nil)

				fakeBoshIOReleaseSource.DownloadReleaseReturns(
					release.Local{ID: boshIOReleaseID, LocalPath: "local-path3", SHA1: "correct-sha"},
					nil)

				fakeLocalReleaseDirectory.GetLocalReleasesReturns(nil, nil)
			})

			It("completes successfully", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())
			})

			It("fetches compiled release from s3 compiled release source", func() {
				Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

				releasesDir, object, threads := fakeS3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(object).To(Equal(
					release.Remote{ID: s3CompiledReleaseID, RemotePath: "some-s3-key", SourceID: s3CompiledReleaseSourceID},
				))
			})

			It("fetches built release from s3 built release source", func() {
				Expect(fakeS3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				releasesDir, object, threads := fakeS3BuiltReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(object).To(Equal(
					release.Remote{ID: s3BuiltReleaseID, RemotePath: "some-other-s3-key", SourceID: s3BuiltReleaseSourceID},
				))
			})

			It("fetches bosh.io release from bosh.io release source", func() {
				Expect(fakeBoshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				releasesDir, object, threads := fakeBoshIOReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(object).To(Equal(
					release.Remote{ID: boshIOReleaseID, RemotePath: "some-bosh-io-url", SourceID: boshIOReleaseSourceID},
				))
			})
		})

		Context("when all releases are already present in releases directory", func() {
			BeforeEach(func() {
				lockContents = `---
releases:
- name: some-release-from-local-dir
  version: "1.2.3"
  remote_source: ` + s3CompiledReleaseSourceID + `
  remote_path: not-used
  sha1: correct-sha
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`

				someLocalReleaseID := release.ID{
					Name:    "some-release-from-local-dir",
					Version: "1.2.3",
				}
				fakeLocalReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{ID: someLocalReleaseID, LocalPath: "/path/to/some/release", SHA1: "correct-sha"},
				}, nil)
			})

			It("no-ops", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
				Expect(fakeS3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
				Expect(fakeBoshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
			})
		})

		Context("when some releases are already present in output directory", func() {
			var (
				missingReleaseS3CompiledID   release.ID
				missingReleaseS3CompiledPath = "s3-key-some-missing-release-on-s3-compiled"
				missingReleaseBoshIOID       release.ID
				missingReleaseBoshIOPath     = "some-other-bosh-io-key"
				missingReleaseS3BuiltID      release.ID
				missingReleaseS3BuiltPath    = "s3-key-some-missing-release-on-s3-built"

				missingReleaseS3Compiled,
				missingReleaseBoshIO,
				missingReleaseS3Built release.Remote
			)
			BeforeEach(func() {
				lockContents = `---
releases:
- name: some-release
  version: "1.2.3"
  remote_source: ` + s3BuiltReleaseSourceID + `
  remote_path: not-used
  sha1: correct-sha
- name: some-tiny-release
  version: "1.2.3"
  remote_source: ` + boshIOReleaseSourceID + `
  remote_path: not-used2
  sha1: correct-sha
- name: some-missing-release-on-s3-compiled
  version: "4.5.6"
  remote_source: ` + s3CompiledReleaseSourceID + `
  remote_path: ` + missingReleaseS3CompiledPath + `
  sha1: correct-sha
- name: some-missing-release-on-boshio
  version: "5.6.7"
  remote_source: ` + boshIOReleaseSourceID + `
  remote_path: ` + missingReleaseBoshIOPath + `
  sha1: correct-sha
- name: some-missing-release-on-s3-built
  version: "8.9.0"
  remote_source: ` + s3BuiltReleaseSourceID + `
  remote_path: ` + missingReleaseS3BuiltPath + `
  sha1: correct-sha
stemcell_criteria:
  os: some-os
  version: "4.5.6"`

				missingReleaseS3CompiledID = release.ID{Name: "some-missing-release-on-s3-compiled", Version: "4.5.6"}
				missingReleaseBoshIOID = release.ID{Name: "some-missing-release-on-boshio", Version: "5.6.7"}
				missingReleaseS3BuiltID = release.ID{Name: "some-missing-release-on-s3-built", Version: "8.9.0"}

				fakeLocalReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{
						ID:        release.ID{Name: "some-release", Version: "1.2.3"},
						LocalPath: "path/to/some/release",
						SHA1:      "correct-sha",
					},
					{
						ID:        release.ID{Name: "some-tiny-release", Version: "1.2.3"},
						LocalPath: "path/to/some/tiny/release",
						SHA1:      "correct-sha",
					},
				}, nil)

				fakeS3CompiledReleaseSource.DownloadReleaseReturns(release.Local{
					ID: missingReleaseS3CompiledID, LocalPath: "local-path-1", SHA1: "correct-sha",
				}, nil)

				fakeBoshIOReleaseSource.DownloadReleaseReturns(release.Local{
					ID: missingReleaseBoshIOID, LocalPath: "local-path-2", SHA1: "correct-sha",
				}, nil)

				fakeS3BuiltReleaseSource.DownloadReleaseReturns(release.Local{
					ID: missingReleaseS3BuiltID, LocalPath: "local-path-3", SHA1: "correct-sha",
				}, nil)

				missingReleaseS3Compiled = release.Remote{ID: missingReleaseS3CompiledID, RemotePath: missingReleaseS3CompiledPath, SourceID: s3CompiledReleaseSourceID}
				missingReleaseBoshIO = release.Remote{ID: missingReleaseBoshIOID, RemotePath: missingReleaseBoshIOPath, SourceID: boshIOReleaseSourceID}
				missingReleaseS3Built = release.Remote{ID: missingReleaseS3BuiltID, RemotePath: missingReleaseS3BuiltPath, SourceID: s3BuiltReleaseSourceID}
			})

			It("downloads only the missing releases", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object, _ := fakeS3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseS3Compiled))

				Expect(fakeBoshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object, _ = fakeBoshIOReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseBoshIO))

				Expect(fakeS3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object, _ = fakeS3BuiltReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseS3Built))
			})

			Context("when download fails", func() {
				var (
					wrappedErr error
				)

				BeforeEach(func() {
					wrappedErr = errors.New("kaboom")
					fakeS3CompiledReleaseSource.DownloadReleaseReturns(
						release.Local{},
						wrappedErr,
					)
				})

				It("returns an error", func() {
					Expect(fetchExecuteErr).To(HaveOccurred())
					Expect(fetchExecuteErr).To(MatchError(ContainSubstring("download failed")))
					Expect(errors.Is(fetchExecuteErr, wrappedErr)).To(BeTrue())
				})
			})

			Context("when the downloaded release has the wrong sha1", func() {
				var badReleasePath string

				BeforeEach(func() {
					badReleasePath = filepath.Join(someReleasesDirectory, "local-path-3")

					fakeS3BuiltReleaseSource.DownloadReleaseCalls(func(string, release.Remote, int) (release.Local, error) {
						f, err := os.Create(badReleasePath)
						Expect(err).NotTo(HaveOccurred())
						defer f.Close()

						return release.Local{
							ID: missingReleaseS3BuiltID, LocalPath: badReleasePath, SHA1: "wrong-sha",
						}, nil
					})
				})

				It("errors", func() {
					Expect(fetchExecuteErr).To(MatchError(ContainSubstring("incorrect SHA1")))
					Expect(fetchExecuteErr).To(MatchError(ContainSubstring(`"correct-sha"`)))
					Expect(fetchExecuteErr).To(MatchError(ContainSubstring(`"wrong-sha"`)))
				})

				It("deletes the release file from disk", func() {
					_, err := os.Stat(badReleasePath)
					Expect(err).To(HaveOccurred())
					Expect(os.IsNotExist(err)).To(BeTrue(), "Expected file %q not to exist, but got a different error: %v", badReleasePath, err)
				})
			})
		})

		Context("when there are extra releases locally that are not in the Kilnfile.lock", func() {
			var (
				boshIOReleaseID = release.ID{Name: "some-release", Version: "1.2.3"}
				localReleaseID  = release.ID{Name: "some-extra-release", Version: "1.2.3"}
			)
			BeforeEach(func() {

				lockContents = `---
releases:
- name: some-release
  version: "1.2.3"
  remote_source: ` + s3CompiledReleaseSourceID + `
  remote_path: not-used
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`
				fakeLocalReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{ID: localReleaseID, LocalPath: "path/to/some/extra/release", SHA1: "correct-sha"},
				}, nil)

				fakeBoshIOReleaseSource.DownloadReleaseReturns(
					release.Local{ID: boshIOReleaseID, LocalPath: "local-path", SHA1: "correct-sha"},
					nil)

			})

			Context("in non-interactive mode", func() {
				BeforeEach(func() {
					fetchExecuteArgs = []string{
						"--releases-directory", someReleasesDirectory,
						"--kilnfile", someKilnfilePath,
						"--no-confirm",
					}
				})

				It("deletes the extra releases", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))

					extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(extras).To(HaveLen(1))
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(ConsistOf(
						release.Local{
							ID:        release.ID{Name: "some-extra-release", Version: "1.2.3"},
							LocalPath: "path/to/some/extra/release",
							SHA1:      "correct-sha",
						},
					))
				})
			})

			Context("when multiple variable files are provided", func() {
				const TemplatizedKilnfileYMLContents = `
---
release_sources:
  - type: s3
    compiled: true
    bucket: $( variable "bucket" )
    region: $( variable "region" )
    access_key_id: $( variable "access_key" )
    secret_access_key: $( variable "secret_key" )
    path_template: $( variable "path_template" )
`

				var (
					someVariableFile, otherVariableFile *os.File
				)

				BeforeEach(func() {
					var err error

					someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
					err = ioutil.WriteFile(someKilnfilePath, []byte(TemplatizedKilnfileYMLContents), 0644)
					Expect(err).NotTo(HaveOccurred())

					someVariableFile, err = ioutil.TempFile(tmpDir, "variables-file1")
					Expect(err).NotTo(HaveOccurred())
					defer someVariableFile.Close()

					variables := map[string]string{
						"bucket": "my-releases",
					}
					data, err := yaml.Marshal(&variables)
					Expect(err).NotTo(HaveOccurred())
					n, err := someVariableFile.Write(data)
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(HaveLen(n))

					otherVariableFile, err = ioutil.TempFile(tmpDir, "variables-file2")
					Expect(err).NotTo(HaveOccurred())
					defer otherVariableFile.Close()

					variables = map[string]string{
						"access_key":    "newkey",
						"secret_key":    "newsecret",
						"path_template": `2.5/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
					}
					data, err = yaml.Marshal(&variables)
					Expect(err).NotTo(HaveOccurred())

					n, err = otherVariableFile.Write(data)
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(HaveLen(n))

					fetchExecuteArgs = []string{
						"--releases-directory", someReleasesDirectory,
						"--kilnfile", someKilnfilePath,
						"--variables-file", someVariableFile.Name(),
						"--variables-file", otherVariableFile.Name(),
						"--variable", "region=north-east-1",
					}
				})

				It("interpolates variables from both files", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())
				})
			})

			Context("when # of download threads is specified", func() {
				BeforeEach(func() {
					fetchExecuteArgs = []string{
						"--releases-directory", someReleasesDirectory,
						"--kilnfile", someKilnfilePath,
						"--download-threads", "10",
					}
				})

				It("passes concurrency parameter to DownloadReleases", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())
					_, _, threads := fakeS3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
					Expect(threads).To(Equal(10))
				})
			})

			Context("failure cases", func() {
				Context("kilnfile is missing", func() {
					It("returns an error", func() {
						badKilnfilePath := filepath.Join(tmpDir, "non-existent-Kilnfile")
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--kilnfile", badKilnfilePath,
						})
						Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("open %s: no such file or directory", badKilnfilePath))))
					})
				})
				Context("# of download threads is not a number", func() {
					It("returns an error", func() {
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--kilnfile", someKilnfilePath,
							"--download-threads", "not-a-number",
						})
						Expect(err).To(MatchError(fmt.Sprintf("invalid value \"not-a-number\" for flag -download-threads: parse error")))
					})
				})

				Context("when local releases cannot be accessed", func() {
					BeforeEach(func() {
						fakeLocalReleaseDirectory.GetLocalReleasesReturns(nil, errors.New("some-error"))
					})
					It("returns an error", func() {
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--kilnfile", someKilnfilePath,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("some-error"))
					})
				})
			})
		})

	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			Expect(fetch.Usage()).To(Equal(jhanda.Usage{
				Description:      "Fetches releases listed in Kilnfile.lock from S3 and downloads it locally",
				ShortDescription: "fetches releases",
				Flags:            fetch.Options,
			}))
		})
	})
})
