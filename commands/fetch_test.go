package commands_test

import (
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
		fetch                 commands.Fetch
		logger                *log.Logger
		tmpDir                string
		someAssetsFilePath    string
		someAssetsLockPath    string
		someReleasesDirectory string
		err                   error
		fakeDownloader        *fakes.Downloader
		fakeReleaseMatcher    *fakes.ReleaseMatcher
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
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {
		BeforeEach(func() {
			fetch = commands.NewFetch(logger, fakeDownloader, fakeReleaseMatcher)
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
					Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
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

				Expect(fakeReleaseMatcher.GetMatchedReleasesCallCount()).To(Equal(1))
				compiledReleases, _ := fakeReleaseMatcher.GetMatchedReleasesArgsForCall(0)
				Expect(compiledReleases).To(Equal(cargo.CompiledReleases{
					Type:            "s3",
					Bucket:          "my-releases",
					Region:          "north-east-1",
					AccessKeyId:     "newkey",
					SecretAccessKey: "newsecret",
					Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
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
					Expect(err).To(MatchError(fmt.Sprintf("invalid value \"not-a-number\" for flag -download-threads: strconv.ParseInt: parsing \"not-a-number\": invalid syntax")))
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
