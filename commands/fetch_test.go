package commands_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	yaml "gopkg.in/yaml.v2"
)

const MinimalAssetsYMLContents = `
---
compiled_releases:
  type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: mykey
  secret_access_key: mysecret
`

const MinimalAssetsLockContents = `
---
releases:
- name: some-release
  version: "1.2.3"
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`

var _ = Describe("Fetch", func() {
	var (
		fetch                     commands.Fetch
		logger                    *log.Logger
		tmpDir                    string
		someAssetsFilePath        string
		someAssetsLockPath        string
		someReleasesDirectory     string
		err                       error
		fakeDownloader            *fakes.Downloader
		fakeReleaseMatcher        *fakes.ReleaseMatcher
		fakeLocalReleaseDirectory *fakes.LocalReleaseDirectory
	)

	BeforeEach(func() {
		logger = log.New(GinkgoWriter, "", 0)

		tmpDir, err = ioutil.TempDir("", "fetch-test")

		someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someAssetsFilePath = filepath.Join(tmpDir, "assets.yml")
		err = ioutil.WriteFile(someAssetsFilePath, []byte(MinimalAssetsYMLContents), 0644)
		Expect(err).NotTo(HaveOccurred())

		someAssetsLockPath = filepath.Join(tmpDir, "assets.lock")
		err = ioutil.WriteFile(someAssetsLockPath, []byte(MinimalAssetsLockContents), 0644)
		Expect(err).NotTo(HaveOccurred())

		fakeDownloader = new(fakes.Downloader)
		fakeReleaseMatcher = new(fakes.ReleaseMatcher)
		fakeReleaseMatcher.GetMatchedReleasesReturns(map[cargo.CompiledRelease]string{
			cargo.CompiledRelease{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "some-s3-key",
		}, nil)

		fakeLocalReleaseDirectory = new(fakes.LocalReleaseDirectory)
		fakeLocalReleaseDirectory.GetLocalReleasesReturns(map[cargo.CompiledRelease]string{}, nil)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {
		BeforeEach(func() {
			fetch = commands.NewFetch(logger, fakeDownloader, fakeReleaseMatcher, fakeLocalReleaseDirectory)
		})
		Context("happy case", func() {
			It("works", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())

				expectedCompiledReleases := cargo.CompiledReleases{
					Type:            "s3",
					Bucket:          "compiled-releases",
					Region:          "us-west-1",
					AccessKeyId:     "mykey",
					SecretAccessKey: "mysecret",
				}
				Expect(fakeReleaseMatcher.GetMatchedReleasesCallCount()).To(Equal(1))
				compiledReleases, assetsLock := fakeReleaseMatcher.GetMatchedReleasesArgsForCall(0)
				Expect(compiledReleases).To(Equal(expectedCompiledReleases))
				Expect(assetsLock).To(Equal(cargo.AssetsLock{
					Releases: []cargo.Release{
						{
							Name:    "some-release",
							Version: "1.2.3",
						},
					},
					Stemcell: cargo.Stemcell{
						OS:      "some-os",
						Version: "4.5.6",
					},
				}))

				Expect(fakeDownloader.DownloadReleasesCallCount()).To(Equal(1))
				releasesDir, compiledReleases, objects, threads := fakeDownloader.DownloadReleasesArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(compiledReleases).To(Equal(expectedCompiledReleases))
				Expect(threads).To(Equal(0))
				Expect(objects).To(HaveKeyWithValue(cargo.CompiledRelease{
					Name:            "some-release",
					Version:         "1.2.3",
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}, "some-s3-key"))
			})
		})

		Context("when all releases are already present in output directory", func() {
			BeforeEach(func() {
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(map[cargo.CompiledRelease]string{
					{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "path/to/some/release"},
					nil)
			})

			It("no-ops", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDownloader.DownloadReleasesCallCount()).To(Equal(1))
				_, _, objects, _ := fakeDownloader.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(0))
			})
		})

		Context("when some releases are already present in output directory", func() {
			BeforeEach(func() {
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(map[cargo.CompiledRelease]string{
					{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "path/to/some/release",
					// a release that has no compiled packages, such as consul-drain, will also have no stemcell criteria in release.MF.
					// we must make sure that we can match this kind of release properly to avoid unnecessary downloads.
					{Name: "some-tiny-release", Version: "1.2.3"}: "path/to/some/tiny/release",
				}, nil)

				fakeReleaseMatcher.GetMatchedReleasesReturns(map[cargo.CompiledRelease]string{
					{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}:         "some-s3-key",
					{Name: "some-tiny-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}:    "some-different-s3-key",
					{Name: "some-missing-release", Version: "4.5.6", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "some-other-s3-key",
				}, nil)
			})

			It("downloads only the missing release", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDownloader.DownloadReleasesCallCount()).To(Equal(1))
				_, _, objects, _ := fakeDownloader.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(HaveKeyWithValue(cargo.CompiledRelease{
					Name:            "some-missing-release",
					Version:         "4.5.6",
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}, "some-other-s3-key"))
			})
		})

		Context("when there are extra releases locally that are not in the assets.lock", func() {
			BeforeEach(func() {
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(map[cargo.CompiledRelease]string{
					{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}:       "path/to/some/release",
					{Name: "some-extra-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "path/to/some/extra/release",
				}, nil)
			})

			Context("in non-interactive mode", func() {
				It("deletes the extra releases", func() {
					err := fetch.Execute([]string{
						"--releases-directory", someReleasesDirectory,
						"--assets-file", someAssetsFilePath,
						"--no-confirm",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDownloader.DownloadReleasesCallCount()).To(Equal(1))
					_, _, objects, _ := fakeDownloader.DownloadReleasesArgsForCall(0)
					Expect(objects).To(HaveLen(0))
					Expect(objects).To(Not(HaveKeyWithValue(cargo.CompiledRelease{
						Name:            "some-extra-release",
						Version:         "1.2.3",
						StemcellOS:      "some-os",
						StemcellVersion: "4.5.6",
					}, "some-other-s3-key")))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					releaseDir, extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(releaseDir).To(Equal(someReleasesDirectory))
					Expect(extras).To(HaveLen(1))
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveKeyWithValue(cargo.CompiledRelease{
						Name:            "some-extra-release",
						Version:         "1.2.3",
						StemcellOS:      "some-os",
						StemcellVersion: "4.5.6",
					}, "path/to/some/extra/release"))
				})
			})

			Context("when multiple variable files are provided", func() {
				var someVariableFile, otherVariableFile *os.File
				const TemplatizedAssetsYMLContents = `
---
compiled_releases:
  type: s3
  bucket: $( variable "bucket" )
  region: $( variable "region" )
  access_key_id: $( variable "access_key" )
  secret_access_key: $( variable "secret_key" )
`

				BeforeEach(func() {
					var err error

					someAssetsFilePath = filepath.Join(tmpDir, "assets.yml")
					err = ioutil.WriteFile(someAssetsFilePath, []byte(TemplatizedAssetsYMLContents), 0644)
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
						"access_key": "newkey",
						"secret_key": "newsecret",
					}
					data, err = yaml.Marshal(&variables)
					Expect(err).NotTo(HaveOccurred())

					n, err = otherVariableFile.Write(data)
					Expect(err).NotTo(HaveOccurred())
					Expect(data).To(HaveLen(n))
				})

				It("interpolates variables from both files", func() {
					err := fetch.Execute([]string{
						"--releases-directory", someReleasesDirectory,
						"--assets-file", someAssetsFilePath,
						"--variables-file", someVariableFile.Name(),
						"--variables-file", otherVariableFile.Name(),
						"--variable", "region=north-east-1",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeReleaseMatcher.GetMatchedReleasesCallCount()).To(Equal(1))
					compiledReleases, _ := fakeReleaseMatcher.GetMatchedReleasesArgsForCall(0)
					Expect(compiledReleases).To(Equal(cargo.CompiledReleases{
						Type:            "s3",
						Bucket:          "my-releases",
						Region:          "north-east-1",
						AccessKeyId:     "newkey",
						SecretAccessKey: "newsecret",
					}))
				})
			})

			Context("when # of download threads is specified", func() {
				It("passes concurrency parameter to DownloadReleases", func() {
					err := fetch.Execute([]string{
						"--releases-directory", someReleasesDirectory,
						"--assets-file", someAssetsFilePath,
						"--download-threads", "10",
					})
					Expect(err).NotTo(HaveOccurred())
					_, _, _, threads := fakeDownloader.DownloadReleasesArgsForCall(0)
					Expect(threads).To(Equal(10))
				})
			})

			Context("failure cases", func() {
				Context("the assets-file flag is missing", func() {
					It("returns a flag error", func() {
						err := fetch.Execute([]string{"--releases-directory", "reldir"})
						Expect(err).To(MatchError("missing required flag \"--assets-file\""))
					})
				})
				Context("the releases-directory flag is missing", func() {
					It("returns a flag error", func() {
						err := fetch.Execute([]string{"--assets-file", "assets.yml"})
						Expect(err).To(MatchError("missing required flag \"--releases-directory\""))
					})
				})
				Context("assets.yml is missing", func() {
					It("returns an error", func() {
						badAssetsFilePath := filepath.Join(tmpDir, "non-existent-assets.yml")
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--assets-file", badAssetsFilePath,
						})
						Expect(err).To(MatchError(fmt.Sprintf("open %s: no such file or directory", badAssetsFilePath)))
					})
				})
				Context("# of download threads is not a number", func() {
					It("returns an error", func() {
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--assets-file", someAssetsFilePath,
							"--download-threads", "not-a-number",
						})
						Expect(err).To(MatchError(fmt.Sprintf("invalid value \"not-a-number\" for flag -download-threads: parse error")))
					})
				})

				Context("when local releases cannot be fetched", func() {
					BeforeEach(func() {
						fakeLocalReleaseDirectory.GetLocalReleasesReturns(nil, errors.New("some-error"))
					})
					It("returns an error", func() {
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--assets-file", someAssetsFilePath,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("some-error"))
					})
				})
			})
		})

		Describe("Usage", func() {
			It("returns usage information for the command", func() {
				Expect(fetch.Usage()).To(Equal(jhanda.Usage{
					Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
					ShortDescription: "fetches releases",
					Flags:            fetch.Options,
				}))
			})
		})
	})
})
