package commands_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	componentFakes "github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
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
		fakeS3CompiledReleaseSource *componentFakes.ReleaseSource
		fakeBoshIOReleaseSource     *componentFakes.ReleaseSource
		fakeS3BuiltReleaseSource    *componentFakes.ReleaseSource
		fakeReleaseSources          *componentFakes.MultiReleaseSource
		releaseSourceList           component.ReleaseSourceList
		fakeLocalReleaseDirectory   *commandsFakes.LocalReleaseDirectory
		multiReleaseSourceProvider  commands.MultiReleaseSourceProvider

		fetchExecuteArgs []string
		fetchExecuteErr  error
	)

	const (
		s3CompiledReleaseSourceID = "s3-compiled"
		s3BuiltReleaseSourceID    = "s3-built"
		boshIOReleaseSourceID     = component.ReleaseSourceTypeBOSHIO
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			fakeReleaseSources = new(componentFakes.MultiReleaseSource)
			logger = log.New(GinkgoWriter, "", 0)

			var err error
			tmpDir, err = os.MkdirTemp("", "fetch-test")
			Expect(err).NotTo(HaveOccurred())

			someReleasesDirectory, err = os.MkdirTemp(tmpDir, "")
			Expect(err).NotTo(HaveOccurred())

			someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
			err = os.WriteFile(someKilnfilePath, []byte(""), 0o644)
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

			fakeS3CompiledReleaseSource = new(componentFakes.ReleaseSource)
			fakeS3CompiledReleaseSource.ConfigurationReturns(cargo.ReleaseSourceConfig{
				ID: s3CompiledReleaseSourceID,
			})
			fakeBoshIOReleaseSource = new(componentFakes.ReleaseSource)
			fakeBoshIOReleaseSource.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: boshIOReleaseSourceID})
			fakeS3BuiltReleaseSource = new(componentFakes.ReleaseSource)
			fakeS3BuiltReleaseSource.ConfigurationReturns(cargo.ReleaseSourceConfig{ID: s3BuiltReleaseSourceID})

			fetchExecuteArgs = []string{
				"--releases-directory", someReleasesDirectory,
				"--kilnfile", someKilnfilePath,
			}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		JustBeforeEach(func() {
			releaseSourceList = component.NewMultiReleaseSource(fakeS3CompiledReleaseSource, fakeBoshIOReleaseSource, fakeS3BuiltReleaseSource)
			fakeReleaseSources.FindByIDStub = func(s string) (component.ReleaseSource, error) {
				return releaseSourceList.FindByID(s)
			}
			fakeReleaseSources.DownloadReleaseStub = func(s string, lock cargo.BOSHReleaseTarballLock) (component.Local, error) {
				return releaseSourceList.DownloadRelease(s, lock)
			}
			fakeReleaseSources.FindReleaseVersionStub = func(requirement cargo.BOSHReleaseTarballSpecification, withSHA bool) (cargo.BOSHReleaseTarballLock, error) {
				return releaseSourceList.FindReleaseVersion(requirement, false)
			}
			fakeReleaseSources.GetMatchedReleaseStub = func(requirement cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
				return releaseSourceList.GetMatchedRelease(requirement)
			}
			multiReleaseSourceProvider = func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) component.MultiReleaseSource {
				return fakeReleaseSources
			}

			err := os.WriteFile(someKilnfileLockPath, []byte(lockContents), 0o644)
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
				releaseID     cargo.BOSHReleaseTarballSpecification
				releaseOnDisk component.Local
			)
			BeforeEach(func() {
				releaseID = cargo.BOSHReleaseTarballSpecification{Name: "some-release", Version: "0.1.0"}
				fakeS3CompiledReleaseSource.DownloadReleaseReturns(
					component.Local{
						Lock:      releaseID.Lock().WithSHA1("correct-sha"),
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
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
					releaseOnDisk = component.Local{
						Lock:      releaseID.Lock().WithSHA1("wrong-sha"),
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
					}
					fakeLocalReleaseDirectory.GetLocalReleasesReturns([]component.Local{releaseOnDisk}, nil)
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
					releaseOnDisk = component.Local{
						Lock:      releaseID.Lock().WithSHA1("correct-sha"),
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
					}
					fakeLocalReleaseDirectory.GetLocalReleasesReturns([]component.Local{releaseOnDisk}, nil)
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
				s3CompiledReleaseID = cargo.BOSHReleaseTarballSpecification{Name: "lts-compiled-release", Version: "1.2.4"}
				s3BuiltReleaseID    = cargo.BOSHReleaseTarballSpecification{Name: "lts-built-release", Version: "1.3.9"}
				boshIOReleaseID     = cargo.BOSHReleaseTarballSpecification{Name: "boshio-release", Version: "1.4.16"}
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
					component.Local{Lock: s3CompiledReleaseID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path"},
					nil)

				fakeS3BuiltReleaseSource.DownloadReleaseReturns(
					component.Local{Lock: s3BuiltReleaseID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path2"},
					nil)

				fakeBoshIOReleaseSource.DownloadReleaseReturns(
					component.Local{Lock: boshIOReleaseID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path3"},
					nil)

				fakeLocalReleaseDirectory.GetLocalReleasesReturns(nil, nil)
			})

			It("completes successfully", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())
			})

			It("fetches compiled release from s3 compiled release source", func() {
				Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

				releasesDir, object := fakeS3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(object).To(Equal(
					s3CompiledReleaseID.Lock().WithRemote(s3CompiledReleaseSourceID, "some-s3-key"),
				))
			})

			It("fetches built release from s3 built release source", func() {
				Expect(fakeS3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				releasesDir, object := fakeS3BuiltReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(object).To(Equal(
					s3BuiltReleaseID.Lock().WithRemote(s3BuiltReleaseSourceID, "some-other-s3-key"),
				))
			})

			It("fetches bosh.io release from bosh.io release source", func() {
				Expect(fakeBoshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				releasesDir, object := fakeBoshIOReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(object).To(Equal(
					boshIOReleaseID.Lock().WithRemote(boshIOReleaseSourceID, "some-bosh-io-url"),
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

				someLocalReleaseID := cargo.BOSHReleaseTarballSpecification{
					Name:    "some-release-from-local-dir",
					Version: "1.2.3",
				}
				fakeLocalReleaseDirectory.GetLocalReleasesReturns([]component.Local{
					{Lock: someLocalReleaseID.Lock().WithSHA1("correct-sha"), LocalPath: "/path/to/some/release"},
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
				missingReleaseS3CompiledID   cargo.BOSHReleaseTarballSpecification
				missingReleaseS3CompiledPath = "s3-key-some-missing-release-on-s3-compiled"
				missingReleaseBoshIOID       cargo.BOSHReleaseTarballSpecification
				missingReleaseBoshIOPath     = "some-other-bosh-io-key"
				missingReleaseS3BuiltID      cargo.BOSHReleaseTarballSpecification
				missingReleaseS3BuiltPath    = "s3-key-some-missing-release-on-s3-built"

				missingReleaseS3Compiled,
				missingReleaseBoshIO,
				missingReleaseS3Built cargo.BOSHReleaseTarballLock
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

				missingReleaseS3CompiledID = cargo.BOSHReleaseTarballSpecification{Name: "some-missing-release-on-s3-compiled", Version: "4.5.6"}
				missingReleaseBoshIOID = cargo.BOSHReleaseTarballSpecification{Name: "some-missing-release-on-boshio", Version: "5.6.7"}
				missingReleaseS3BuiltID = cargo.BOSHReleaseTarballSpecification{Name: "some-missing-release-on-s3-built", Version: "8.9.0"}

				fakeLocalReleaseDirectory.GetLocalReleasesReturns([]component.Local{
					{
						Lock:      cargo.BOSHReleaseTarballLock{Name: "some-release", Version: "1.2.3", SHA1: "correct-sha"},
						LocalPath: "path/to/some/release",
					},
					{
						Lock:      cargo.BOSHReleaseTarballLock{Name: "some-tiny-release", Version: "1.2.3", SHA1: "correct-sha"},
						LocalPath: "path/to/some/tiny/release",
					},
				}, nil)

				fakeS3CompiledReleaseSource.DownloadReleaseReturns(component.Local{
					Lock: missingReleaseS3CompiledID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path-1",
				}, nil)

				fakeBoshIOReleaseSource.DownloadReleaseReturns(component.Local{
					Lock: missingReleaseBoshIOID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path-2",
				}, nil)

				fakeS3BuiltReleaseSource.DownloadReleaseReturns(component.Local{
					Lock: missingReleaseS3BuiltID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path-3",
				}, nil)

				missingReleaseS3Compiled = missingReleaseS3CompiledID.Lock().WithRemote(s3CompiledReleaseSourceID, missingReleaseS3CompiledPath)
				missingReleaseBoshIO = missingReleaseBoshIOID.Lock().WithRemote(boshIOReleaseSourceID, missingReleaseBoshIOPath)
				missingReleaseS3Built = missingReleaseS3BuiltID.Lock().WithRemote(s3BuiltReleaseSourceID, missingReleaseS3BuiltPath)
			})

			It("downloads only the missing releases", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object := fakeS3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseS3Compiled))

				Expect(fakeBoshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object = fakeBoshIOReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseBoshIO))

				Expect(fakeS3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object = fakeS3BuiltReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseS3Built))
			})

			Context("when download fails", func() {
				var wrappedErr error

				BeforeEach(func() {
					wrappedErr = errors.New("kaboom")
					fakeS3CompiledReleaseSource.DownloadReleaseReturns(
						component.Local{},
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

					fakeS3BuiltReleaseSource.DownloadReleaseCalls(func(string, cargo.BOSHReleaseTarballLock) (component.Local, error) {
						f, err := os.Create(badReleasePath)
						Expect(err).NotTo(HaveOccurred())
						defer closeAndIgnoreError(f)

						return component.Local{
							Lock: missingReleaseS3BuiltID.Lock().WithSHA1("wrong-sha"), LocalPath: badReleasePath,
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
				boshIOReleaseID = cargo.BOSHReleaseTarballSpecification{Name: "some-release", Version: "1.2.3"}
				localReleaseID  = cargo.BOSHReleaseTarballSpecification{Name: "some-extra-release", Version: "1.2.3"}
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
				fakeLocalReleaseDirectory.GetLocalReleasesReturns([]component.Local{
					{Lock: localReleaseID.Lock().WithSHA1("correct-sha"), LocalPath: "path/to/some/extra/release"},
				}, nil)

				fakeBoshIOReleaseSource.DownloadReleaseReturns(
					component.Local{Lock: boshIOReleaseID.Lock().WithSHA1("correct-sha"), LocalPath: "local-path"},
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
						component.Local{
							Lock:      cargo.BOSHReleaseTarballLock{Name: "some-extra-release", Version: "1.2.3", SHA1: "correct-sha"},
							LocalPath: "path/to/some/extra/release",
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

				var someVariableFile, otherVariableFile *os.File

				BeforeEach(func() {
					var err error

					someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
					err = os.WriteFile(someKilnfilePath, []byte(TemplatizedKilnfileYMLContents), 0o644)
					Expect(err).NotTo(HaveOccurred())

					someVariableFile, err = os.CreateTemp(tmpDir, "variables-file1")
					Expect(err).NotTo(HaveOccurred())
					defer closeAndIgnoreError(someVariableFile)

					variables := map[string]string{
						"bucket": "my-releases",
					}
					data, err := yaml.Marshal(&variables)
					Expect(err).NotTo(HaveOccurred())
					n, err := someVariableFile.Write(data)
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(HaveLen(n))

					otherVariableFile, err = os.CreateTemp(tmpDir, "variables-file2")
					Expect(err).NotTo(HaveOccurred())
					defer closeAndIgnoreError(otherVariableFile)

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
					// Expect(fakeReleaseSources.SetDownloadThreadsCallCount()).To(Equal(1))
					// Expect(fakeReleaseSources.SetDownloadThreadsArgsForCall(0)).To(Equal(50))
					// TODO: add test back in
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
						Expect(err).To(MatchError(`invalid value "not-a-number" for flag -download-threads: parse error`))
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
				Description:      "Fetches releases in Kilnfile.lock from sources and save in releases directory locally",
				ShortDescription: "fetches releases",
				Flags:            commands.FetchOptions{},
			}))
		})
	})
})
