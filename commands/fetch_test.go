package commands_test

import (
	"errors"
	"fmt"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/fetcher"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"
)

const MinimalAssetsYMLContents = `
---
compiled_releases:
  type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: mykey
  secret_access_key: mysecret
  regex: ^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$
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
		fetch                       commands.Fetch
		logger                      *log.Logger
		tmpDir                      string
		someAssetsFilePath          string
		someAssetsLockPath          string
		assetsLockContents          string
		someReleasesDirectory       string
		err                         error
		fakeS3CompiledReleaseSource *fakes.ReleaseSource
		fakeBoshIOReleaseSource     *fakes.ReleaseSource
		fakeS3BuiltReleaseSource    *fakes.ReleaseSource
		fakeReleaseSources          []commands.ReleaseSource
		fakeLocalReleaseDirectory   *fakes.LocalReleaseDirectory
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
		assetsLockContents = MinimalAssetsLockContents

		fakeS3CompiledReleaseSource = new(fakes.ReleaseSource)
		fakeBoshIOReleaseSource = new(fakes.ReleaseSource)
		fakeS3BuiltReleaseSource = new(fakes.ReleaseSource)
		fakeReleaseSources = []commands.ReleaseSource{fakeS3CompiledReleaseSource, fakeBoshIOReleaseSource, fakeS3BuiltReleaseSource}
		fakeS3CompiledReleaseSource.GetMatchedReleasesReturns(fetcher.CompiledReleaseSet{
			{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "some-s3-key",
		}, nil)

		fakeLocalReleaseDirectory = new(fakes.LocalReleaseDirectory)
		fakeLocalReleaseDirectory.GetLocalReleasesReturns(fetcher.CompiledReleaseSet{}, nil)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {
		JustBeforeEach(func() {
			err := ioutil.WriteFile(someAssetsLockPath, []byte(assetsLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())
			releaseSourcesFactory := func(cargo.Assets) []commands.ReleaseSource { return fakeReleaseSources }
			fetch = commands.NewFetch(logger, releaseSourcesFactory, fakeLocalReleaseDirectory)
		})

		// no local releases, some releases found in `compiled-releases` S3 bucket
		// no local releases, some releases found in bosh.io
		// no local releases, remaining releases fetched in `uncompiled-releases` S3 bucket
		Context("happy case", func() {
			It("works", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.GetMatchedReleasesCallCount()).To(Equal(1))
				desiredReleaseSet := fakeS3CompiledReleaseSource.GetMatchedReleasesArgsForCall(0)
				Expect(desiredReleaseSet).To(Equal(fetcher.CompiledReleaseSet{
					{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "",
				}))

				Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				releasesDir, objects, threads := fakeS3CompiledReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(objects).To(HaveKeyWithValue(fetcher.CompiledRelease{
					Name:            "some-release",
					Version:         "1.2.3",
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}, "some-s3-key"))
			})
		})

		Context("when one or more releases are not available on S3(compiled), bosh.io, nor S3(built)", func() {
			BeforeEach(func() {
				emptyReleaseSet := map[fetcher.CompiledRelease]string{}

				fakeS3CompiledReleaseSource.GetMatchedReleasesReturns(emptyReleaseSet, nil)
				fakeBoshIOReleaseSource.GetMatchedReleasesReturns(emptyReleaseSet, nil)
				//fakeS3CompiledReleaseSource.GetMatchedReleasesReturns(emptyReleaseSet, nil)
			})
			It("reports an error", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp(`Name:some-release Version:1.2.3 StemcellOS:some-os StemcellVersion:4.5.6`))
			})
		})

		// no local releases, all releases found in `compiled-releases` S3 bucket
		// no local releases, all releases found in bosh.io
		// no local releases, all releases found in `uncompiled-releases` S3 bucket

		// no local releases, some releases found in `compiled-releases` S3 bucket
		// no local releases, remaining releases found in bosh.io
		// no local releases, no releases fetched in `uncompiled-releases` S3 bucket

		// some local releases, all releases found in `compiled-releases` S3 bucket
		// some local releases, all releases found in bosh.io
		// some local releases, all releases found in `uncompiled-releases` S3 bucket

		// all local releases, no releases fetched from `compiled-releases` S3 bucket
		// all local releases, no releases fetched from bosh.io
		// all local releases, no releases fetched from `uncompiled-releases` S3 bucket

		Context("when all releases are already present in output directory", func() {
			BeforeEach(func() {
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(map[fetcher.CompiledRelease]string{
					{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "path/to/some/release"},
					nil)
			})

			It("no-ops", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
				Expect(fakeBoshIOReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
			})
		})

		Context("when some releases are already present in output directory", func() {
			var (
				missingReleaseS3Compiled     fetcher.CompiledRelease
				missingReleaseS3CompiledPath = "s3-key-some-missing-release-on-s3-compiled"
				missingReleaseBoshIO         fetcher.CompiledRelease
				missingReleaseBoshIOPath     = "some-other-bosh-io-key"
				missingReleaseS3Built        fetcher.CompiledRelease
				missingReleaseS3BuiltPath    = "s3-key-some-missing-release-on-s3-built"
			)
			BeforeEach(func() {
				assetsLockContents = `---
releases:
- name: some-release
  version: "1.2.3"
- name: some-tiny-release
  version: "1.2.3"
- name: some-missing-release-on-s3-compiled
  version: "4.5.6"
- name: some-missing-release-on-boshio
  version: "5.6.7"
- name: some-missing-release-on-s3-built
  version: "8.9.0"
stemcell_criteria:
  os: some-os
  version: "4.5.6"`

				missingReleaseS3Compiled = fetcher.CompiledRelease{Name: "some-missing-release-on-s3-compiled", Version: "4.5.6", StemcellOS: "some-os", StemcellVersion: "4.5.6"}
				missingReleaseBoshIO = fetcher.CompiledRelease{Name: "some-missing-release-on-boshio", Version: "5.6.7", StemcellOS: "some-os", StemcellVersion: "4.5.6"}
				missingReleaseS3Built = fetcher.CompiledRelease{Name: "some-missing-release-on-s3-built", Version: "8.9.0"}
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(fetcher.CompiledReleaseSet{
					fetcher.CompiledRelease{Name: "some-release", Version: "1.2.3", StemcellOS: "some-os", StemcellVersion: "4.5.6"}: "path/to/some/release",
					// a release that has no compiled packages, such as consul-drain, will also have no stemcell criteria in release.MF.
					// we must make sure that we can match this kind of release properly to avoid unnecessary downloads.
					{Name: "some-tiny-release", Version: "1.2.3"}: "path/to/some/tiny/release",
				}, nil)

				fakeS3CompiledReleaseSource.GetMatchedReleasesReturns(
					fetcher.CompiledReleaseSet{
						missingReleaseS3Compiled: missingReleaseS3CompiledPath,
					},
					nil,
				)
				fakeBoshIOReleaseSource.GetMatchedReleasesReturns(
					fetcher.CompiledReleaseSet{
						missingReleaseBoshIO: missingReleaseBoshIOPath,
					},
					nil,
				)
				fakeS3BuiltReleaseSource.GetMatchedReleasesReturns(
					fetcher.CompiledReleaseSet{
						missingReleaseS3Built: missingReleaseS3BuiltPath,
					},
					nil,
				)
			})

			It("downloads only the missing releases", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())
				//_ = err

				Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				_, objects, _ := fakeS3CompiledReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(HaveKeyWithValue(missingReleaseS3Compiled, missingReleaseS3CompiledPath))

				Expect(fakeBoshIOReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				_, objects, _ = fakeBoshIOReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(HaveKeyWithValue(missingReleaseBoshIO, missingReleaseBoshIOPath))

				Expect(fakeS3BuiltReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				_, objects, _ = fakeS3BuiltReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(HaveKeyWithValue(missingReleaseS3Built, missingReleaseS3BuiltPath))
			})

			Context("when download fails", func() {
				BeforeEach(func() {
					fakeS3CompiledReleaseSource.DownloadReleasesReturns(
						errors.New("download failed"),
					)
				})

				It("returns an error", func() {
					err := fetch.Execute([]string{
						"--releases-directory", someReleasesDirectory,
						"--assets-file", someAssetsFilePath,
					})
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when there are extra releases locally that are not in the assets.lock", func() {
			BeforeEach(func() {
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(map[fetcher.CompiledRelease]string{
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

					Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					releaseDir, extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(releaseDir).To(Equal(someReleasesDirectory))
					Expect(extras).To(HaveLen(1))
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveKeyWithValue(fetcher.CompiledRelease{
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
  regex: $( variable "regex" )
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
						"regex":      `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
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

					Expect(fakeS3CompiledReleaseSource.GetMatchedReleasesCallCount()).To(Equal(1))
					_ = fakeS3CompiledReleaseSource.GetMatchedReleasesArgsForCall(0)
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
					_, _, threads := fakeS3CompiledReleaseSource.DownloadReleasesArgsForCall(0)
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

				Context("when local releases cannot be accessed", func() {
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
