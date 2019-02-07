package commands_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
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

const MinimalAssetsLockContents = `---`

var _ = Describe("Fetch", func() {
	var (
		fetch                 commands.Fetch
		logger                *log.Logger
		tmpDir                string
		someAssetsFilePath    string
		someAssetsLockPath    string
		someReleasesDirectory string
		err                   error
		fakeS3ClientProvider  func(*session.Session, ...*aws.Config) s3iface.S3API
		sessionArg            *session.Session
		fakeS3Client          *fakes.S3Client
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

		fakeS3Client = new(fakes.S3Client)

		fakeS3ClientProvider = func(sess *session.Session, cfgs ...*aws.Config) s3iface.S3API {
			sessionArg = sess
			return fakeS3Client
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {
		BeforeEach(func() {
			fetch = commands.NewFetch(logger, fakeS3ClientProvider)
		})
		Context("happy case", func() {
			It("works", func() {
				err := fetch.Execute([]string{
					"--releases-directory", someReleasesDirectory,
					"--assets-file", someAssetsFilePath,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeS3Client.ListObjectsPagesCallCount()).To(Equal(1))
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
				creds, err := sessionArg.Config.Credentials.Get()
				Expect(err).NotTo(HaveOccurred())
				Expect(creds.AccessKeyID).To(Equal("newkey"))
				Expect(creds.SecretAccessKey).To(Equal("newsecret"))
				Expect(sessionArg.Config.Region).To(Equal(aws.String("north-east-1")))
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
			Context("required files are missing", func() {
				It("returns an error when assets.yml file isn't present", func() {
					badAssetsFilePath := filepath.Join(tmpDir, "non-existent-assets.yml")
					err := fetch.Execute([]string{
						"--releases-directory", someReleasesDirectory,
						"--assets-file", badAssetsFilePath,
					})
					Expect(err).To(MatchError(fmt.Sprintf("open %s: no such file or directory", badAssetsFilePath)))
				})
			})
		})
	})

	Describe("CompiledReleasesRegexp", func() {
		var (
			compiledRelease cargo.CompiledRelease
			regex           *commands.CompiledReleasesRegexp
			err             error
		)

		It("takes a regex string and converts it to a CompiledRelease", func() {
			regex, err = commands.NewCompiledReleasesRegexp(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
			Expect(err).NotTo(HaveOccurred())

			compiledRelease, err = regex.Convert("2.5/uaa/uaa-1.2.3-ubuntu-trusty-123.tgz")
			Expect(err).NotTo(HaveOccurred())
			Expect(compiledRelease).To(Equal(cargo.CompiledRelease{Name: "uaa", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "123"}))
		})

		It("returns an error if s3 key does not match the regex", func() {
			regex, err = commands.NewCompiledReleasesRegexp(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
			Expect(err).NotTo(HaveOccurred())

			compiledRelease, err = regex.Convert("2.5/uaa/uaa-1.2.3-123.tgz")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("s3 key does not match regex"))
		})

		It("returns an error if a capture is missing", func() {
			regex, err = commands.NewCompiledReleasesRegexp(`^2.5/.+/([a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("release_name, release_version, stemcell_os, stemcell_version")))
		})
	})

	Describe("GetMatchedReleases", func() {
		var (
			bucket string
			err    error
		)

		BeforeEach(func() {
			bucket = "some-bucket"
			Expect(err).NotTo(HaveOccurred())
			fakeS3Client = new(fakes.S3Client)
		})

		It("lists all objects that match the given regex", func() {
			key1 := "some-key"
			key2 := "1.10/uaa/uaa-1.2.3-ubuntu-xenial-190.0.0.tgz"
			key3 := "2.5/bpm/bpm-1.2.3-ubuntu-xenial-190.0.0.tgz"
			fakeS3Client.ListObjectsPagesStub = func(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
				shouldContinue := fn(&s3.ListObjectsOutput{
					Contents: []*s3.Object{
						{Key: &key1},
						{Key: &key2},
						{Key: &key3},
					},
				},
					true,
				)
				Expect(shouldContinue).To(BeTrue())
				return nil
			}

			assetsLock := cargo.AssetsLock{
				Releases: []cargo.Release{
					{Name: "bpm", Version: "1.2.3"},
				},
				Stemcell: cargo.Stemcell{
					OS:      "ubuntu-xenial",
					Version: "190.0.0",
				},
			}

			compiledRegex, err := commands.NewCompiledReleasesRegexp(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
			Expect(err).NotTo(HaveOccurred())

			matchedS3Objects, err := commands.GetMatchedReleases(bucket, compiledRegex, fakeS3Client, assetsLock)
			Expect(err).NotTo(HaveOccurred())

			input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
			Expect(input.Bucket).To(Equal(aws.String("some-bucket")))

			Expect(matchedS3Objects).To(HaveLen(1))
			Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, key3))
		})

		It("returns error if any do not match what's in asset.lock", func() {
			key1 := "1.10/uaa/uaa-1.2.3-ubuntu-xenial-190.0.0.tgz"
			key2 := "2.5/bpm/bpm-1.2.3-ubuntu-xenial-190.0.0.tgz"
			fakeS3Client.ListObjectsPagesStub = func(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
				shouldContinue := fn(&s3.ListObjectsOutput{
					Contents: []*s3.Object{
						{Key: &key1},
						{Key: &key2},
					},
				},
					true,
				)
				Expect(shouldContinue).To(BeTrue())
				return nil
			}

			compiledRegex, err := commands.NewCompiledReleasesRegexp(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
			Expect(err).NotTo(HaveOccurred())

			assetsLock := cargo.AssetsLock{
				Releases: []cargo.Release{
					{Name: "bpm", Version: "1.2.3"},
					{Name: "some-release", Version: "1.2.3"},
					{Name: "another-missing-release", Version: "4.5.6"},
				},
				Stemcell: cargo.Stemcell{
					OS:      "ubuntu-xenial",
					Version: "190.0.0",
				},
			}
			_, err = commands.GetMatchedReleases(bucket, compiledRegex, fakeS3Client, assetsLock)
			Expect(err).To(MatchError(`Expected releases were not matched by the regex:
{Name:some-release Version:1.2.3 StemcellOS:ubuntu-xenial StemcellVersion:190.0.0}
{Name:another-missing-release Version:4.5.6 StemcellOS:ubuntu-xenial StemcellVersion:190.0.0}`))

			input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
			Expect(input.Bucket).To(Equal(aws.String("some-bucket")))
		})
	})

	Describe("DownloadReleases", func() {
		var (
			assetsLock       cargo.AssetsLock
			bucket           string
			matchedS3Objects map[cargo.CompiledRelease]string
			fileCreator      func(string) (io.WriterAt, error)
			fakeDownloader   *fakes.Downloader
			fakeBPMFile      *os.File
			fakeUAAFile      *os.File
			bpmInput         *s3.GetObjectInput
			uaaInput         *s3.GetObjectInput
			err              error
		)

		BeforeEach(func() {
			assetsLock = cargo.AssetsLock{
				Releases: []cargo.Release{
					{Name: "uaa", Version: "1.2.3"},
					{Name: "bpm", Version: "1.2.3"},
				},
				Stemcell: cargo.Stemcell{OS: "ubuntu-trusty", Version: "1234"},
			}
			bucket = "some-bucket"

			matchedS3Objects = make(map[cargo.CompiledRelease]string)
			matchedS3Objects[cargo.CompiledRelease{Name: "uaa", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-uaa-key"
			matchedS3Objects[cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-bpm-key"

			fakeBPMFile, err = ioutil.TempFile("", "bpm-release")
			Expect(err).NotTo(HaveOccurred())
			fakeUAAFile, err = ioutil.TempFile("", "uaa-release")
			Expect(err).NotTo(HaveOccurred())

			fileCreator = func(filepath string) (io.WriterAt, error) {
				if strings.Contains(filepath, "uaa") {
					return fakeUAAFile, nil
				} else if strings.Contains(filepath, "bpm") {
					return fakeBPMFile, nil
				}

				return nil, errors.New("unknown filepath")
			}

			bpmInput = &s3.GetObjectInput{Bucket: aws.String("some-bucket"), Key: aws.String("some-bpm-key")}
			uaaInput = &s3.GetObjectInput{Bucket: aws.String("some-bucket"), Key: aws.String("some-uaa-key")}

			fakeDownloader = new(fakes.Downloader)
		})

		AfterEach(func() {
			Expect(os.Remove(fakeBPMFile.Name())).To(Succeed())
			Expect(os.Remove(fakeUAAFile.Name())).To(Succeed())
		})

		It("downloads the appropriate versions of releases listed in the assets.lock", func() {
			err = commands.DownloadReleases(logger, assetsLock, bucket, matchedS3Objects, fileCreator, fakeDownloader, 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeDownloader.DownloadCallCount()).To(Equal(2))

			w1, input1, opts := fakeDownloader.DownloadArgsForCall(0)
			Expect(w1).To(Equal(fakeUAAFile))
			Expect(input1).To(Equal(uaaInput))
			Expect(opts).To(HaveLen(1))

			downloader := &s3manager.Downloader{
				Concurrency: s3manager.DefaultDownloadConcurrency,
			}

			opts[0](downloader)

			Expect(downloader.Concurrency).To(Equal(7))

			w2, input2, opts := fakeDownloader.DownloadArgsForCall(1)
			Expect(w2).To(Equal(fakeBPMFile))
			Expect(input2).To(Equal(bpmInput))
			Expect(opts).To(HaveLen(1))
		})

		Context("when number of threads is not specified", func() {
			It("uses the s3manager package's default download concurrency", func() {
				err = commands.DownloadReleases(logger, assetsLock, bucket, matchedS3Objects, fileCreator, fakeDownloader, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeDownloader.DownloadCallCount()).To(Equal(2))

				w1, input1, opts := fakeDownloader.DownloadArgsForCall(0)
				Expect(w1).To(Equal(fakeUAAFile))
				Expect(input1).To(Equal(uaaInput))
				Expect(opts).To(HaveLen(1))

				downloader := &s3manager.Downloader{
					Concurrency: s3manager.DefaultDownloadConcurrency,
				}

				opts[0](downloader)

				Expect(downloader.Concurrency).To(Equal(s3manager.DefaultDownloadConcurrency))
			})
		})

		It("returns an error if the release does not exist", func() {
			assetsLock.Releases = []cargo.Release{
				{Name: "not-real", Version: "1.2.3"},
			}

			err = commands.DownloadReleases(logger, assetsLock, bucket, matchedS3Objects, fileCreator, fakeDownloader, 0)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("Compiled release: not-real, version: 1.2.3, stemcell OS: ubuntu-trusty, stemcell version: 1234, not found"))
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
