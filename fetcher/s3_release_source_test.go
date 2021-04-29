package fetcher_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/extensions/table"
	"github.com/pivotal-cf/kiln/internal/cargo"

	. "github.com/onsi/gomega/gstruct"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/release"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/aws/aws-sdk-go/service/s3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/fetcher/fakes"
)

func verifySetsConcurrency(opts []func(*s3manager.Downloader), concurrency int) {
	Expect(opts).To(HaveLen(1))

	downloader := &s3manager.Downloader{
		Concurrency: 1,
	}

	opts[0](downloader)

	Expect(downloader.Concurrency).To(Equal(concurrency))
}

var _ = Describe("S3ReleaseSource", func() {
	const (
		sourceID = "s3-source"
	)

	Describe("S3ReleaseSourceFromConfig", func() {
		var (
			config *cargo.ReleaseSourceConfig
			logger *log.Logger
		)

		BeforeEach(func() {
			config = &cargo.ReleaseSourceConfig{
				Bucket:          "my-bucket",
				PathTemplate:    "my-path-template",
				Region:          "my-region",
				AccessKeyId:     "my-access-key",
				SecretAccessKey: "my-secret",
			}
			logger = log.New(GinkgoWriter, "", 0)
		})

		DescribeTable("bad config", func(before func(sourceConfig *cargo.ReleaseSourceConfig), expectedSubstring string) {
			before(config)

			var r interface{}
			func() {
				defer func() {
					r = recover()
				}()
				S3ReleaseSourceFromConfig(*config, logger)
			}()

			Expect(r).To(ContainSubstring(expectedSubstring))
		},
			Entry("path_template is missing",
				func(c *cargo.ReleaseSourceConfig) { c.PathTemplate = "" },
				"path_template",
			),

			Entry("bucket is missing",
				func(c *cargo.ReleaseSourceConfig) { c.Bucket = "" },
				"bucket",
			),
		)
	})

	Describe("DownloadReleases", func() {
		const (
			bucket = "some-bucket"
		)

		var (
			releaseSource         S3ReleaseSource
			logger                *log.Logger
			releaseDir            string
			remoteRelease         release.Remote
			expectedLocalFilename string
			releaseID             release.ID
			fakeS3Downloader      *fakes.S3Downloader
		)

		BeforeEach(func() {
			var err error

			releaseDir, err = ioutil.TempDir("", "kiln-releaseSource-test")
			Expect(err).NotTo(HaveOccurred())

			releaseID = release.ID{Name: "uaa", Version: "1.2.3"}
			remoteRelease = release.Remote{ID: releaseID, RemotePath: "2.10/uaa/uaa-1.2.3-ubuntu-xenial-621.55.tgz", SourceID: bucket}
			expectedLocalFilename = filepath.Base(remoteRelease.RemotePath)

			logger = log.New(GinkgoWriter, "", 0)
			fakeS3Downloader = new(fakes.S3Downloader)
			// fakeS3Downloader writes the given S3 bucket and key into the output file for easy verification
			fakeS3Downloader.DownloadStub = func(writer io.WriterAt, objectInput *s3.GetObjectInput, setConcurrency ...func(dl *s3manager.Downloader)) (int64, error) {
				n, err := writer.WriteAt([]byte(fmt.Sprintf("%s/%s", *objectInput.Bucket, *objectInput.Key)), 0)
				return int64(n), err
			}
			releaseSource = NewS3ReleaseSource(sourceID, bucket, "", false, nil, fakeS3Downloader, nil, logger)
		})

		AfterEach(func() {
			_ = os.RemoveAll(releaseDir)
		})

		It("downloads the appropriate versions of built releases listed in remoteReleases", func() {
			localRelease, err := releaseSource.DownloadRelease(releaseDir, remoteRelease, 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(1))

			releasePath := filepath.Join(releaseDir, expectedLocalFilename)
			releaseContents, err := ioutil.ReadFile(releasePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(releaseContents).To(Equal([]byte("some-bucket/" + remoteRelease.RemotePath)))

			sha1, err := CalculateSum(releasePath, osfs.New(""))
			Expect(err).NotTo(HaveOccurred())

			_, _, opts := fakeS3Downloader.DownloadArgsForCall(0)
			verifySetsConcurrency(opts, 7)

			Expect(localRelease).To(Equal(release.Local{ID: releaseID, LocalPath: releasePath, SHA1: sha1}))
		})

		Context("when number of threads is not specified", func() {
			It("uses the s3manager package's default download concurrency", func() {
				_, err := releaseSource.DownloadRelease(releaseDir, remoteRelease, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(1))

				_, _, opts := fakeS3Downloader.DownloadArgsForCall(0)
				verifySetsConcurrency(opts, s3manager.DefaultDownloadConcurrency)
			})
		})

		Context("failure cases", func() {
			Context("when a file can't be created", func() {
				It("returns an error", func() {
					_, err := releaseSource.DownloadRelease("/non-existent-folder", remoteRelease, 0)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("/non-existent-folder"))
				})
			})

			Context("when a file can't be downloaded", func() {
				BeforeEach(func() {
					fakeS3Downloader.DownloadCalls(func(w io.WriterAt, i *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (int64, error) {
						return 0, errors.New("503 Service Unavailable")
					})
				})

				It("returns an error", func() {
					_, err := releaseSource.DownloadRelease(releaseDir, remoteRelease, 0)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("failed to download file: 503 Service Unavailable\n"))
				})
			})
		})
	})

	Describe("GetMatchedReleases", func() {
		const bucket = "built-bucket"

		var (
			releaseSource  S3ReleaseSource
			fakeS3Client   *fakes.S3Client
			desiredRelease release.Requirement
			bpmReleaseID   release.ID
			bpmKey         string
			logger         *log.Logger
		)

		BeforeEach(func() {
			bpmReleaseID = release.ID{Name: "bpm-release", Version: "1.2.3"}
			desiredRelease = release.Requirement{
				Name:            "bpm-release",
				Version:         "1.2.3",
				StemcellOS:      "ubuntu-xenial",
				StemcellVersion: "190.0.0",
			}

			fakeS3Client = new(fakes.S3Client)
			fakeS3Client.HeadObjectReturns(new(s3.HeadObjectOutput), nil)

			logger = log.New(nil, "", 0)

			releaseSource = NewS3ReleaseSource(
				sourceID,
				bucket,
				`2.5/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
				false,
				fakeS3Client,
				nil,
				nil,
				logger,
			)
			bpmKey = "2.5/bpm/bpm-release-1.2.3-ubuntu-xenial-190.0.0.tgz"
		})

		It("searches for the requested release", func() {
			remoteRelease, found, err := releaseSource.GetMatchedRelease(desiredRelease)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(fakeS3Client.HeadObjectCallCount()).To(Equal(1))
			input := fakeS3Client.HeadObjectArgsForCall(0)
			Expect(input.Bucket).To(PointTo(BeEquivalentTo(bucket)))
			Expect(input.Key).To(PointTo(BeEquivalentTo(bpmKey)))

			Expect(remoteRelease).To(Equal(release.Remote{
				ID:         bpmReleaseID,
				RemotePath: bpmKey,
				SourceID:   sourceID,
			}))
		})

		When("the requested releases doesn't exist in the bucket", func() {
			BeforeEach(func() {
				notFoundError := new(fakes.S3RequestFailure)
				notFoundError.StatusCodeReturns(404)
				fakeS3Client.HeadObjectReturns(nil, notFoundError)
			})

			It("returns not found", func() {
				_, found, err := releaseSource.GetMatchedRelease(desiredRelease)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		When("there is an error evaluating the path template", func() {
			BeforeEach(func() {
				releaseSource = NewS3ReleaseSource(
					sourceID,
					bucket,
					`{{.NoSuchField}}`,
					false,
					fakeS3Client,
					nil,
					nil,
					logger,
				)
			})

			It("returns a descriptive error", func() {
				_, found, err := releaseSource.GetMatchedRelease(desiredRelease)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("FindReleaseVersion from S3 pcf-final-bosh-releases", func() {
		const bucket = "pcf-final-bosh-releases"

		var (
			releaseSource  S3ReleaseSource
			fakeS3Client   *fakes.S3Client
			desiredRelease release.Requirement
			releaseID      release.ID
			uaaKey         string
			logger         *log.Logger
		)
		When("version is semantic and release has version constraint", func() {
			BeforeEach(func() {

				releaseID = release.ID{Name: "uaa", Version: "1.1.1"}
				desiredRelease = release.Requirement{
					Name:              "uaa",
					VersionConstraint: "~1.1",
					StemcellVersion:   "621.71",
				}

				fakeS3Client = new(fakes.S3Client)
				object1Key := "uaa/uaa-1.2.2.tgz"
				object2Key := "uaa/uaa-1.2.3.tgz"
				object3Key := "uaa/uaa-1.1.1.tgz"
				fakeS3Client.ListObjectsV2Returns(&s3.ListObjectsV2Output{
					Contents: []*s3.Object{
						{Key: &object1Key},
						{Key: &object3Key},
						{Key: &object2Key},
					},
				}, nil)

				fakeS3Downloader := new(fakes.S3Downloader)
				// fakeS3Downloader writes the given S3 bucket and key into the output file for easy verification
				fakeS3Downloader.DownloadStub = func(writer io.WriterAt, objectInput *s3.GetObjectInput, setConcurrency ...func(dl *s3manager.Downloader)) (int64, error) {
					n, err := writer.WriteAt([]byte(fmt.Sprintf("%s/%s", *objectInput.Bucket, *objectInput.Key)), 0)
					return int64(n), err
				}

				logger = log.New(GinkgoWriter, "", 0)

				releaseSource = NewS3ReleaseSource(
					sourceID,
					bucket,
					`{{.Name}}/{{.Name}}-{{.Version}}.tgz`,
					false,
					fakeS3Client,
					fakeS3Downloader,
					nil,
					logger,
				)
				uaaKey = "uaa/uaa-1.1.1.tgz"
			})

			It("gets the version that satisfies the constraint", func() {
				remoteRelease, found, err := releaseSource.FindReleaseVersion(desiredRelease)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(fakeS3Client.ListObjectsV2CallCount()).To(Equal(1))
				input := fakeS3Client.ListObjectsV2ArgsForCall(0)
				Expect(*input.Prefix).To(Equal("uaa/"))

				Expect(remoteRelease).To(Equal(release.Remote{
					ID:         releaseID,
					RemotePath: uaaKey,
					SourceID:   sourceID,
					SHA:        "1a77ff749f0f2f49493eb8a517fb7eaa04df9b62",
				}))
			})
		})

		When("version is a single number", func() {
			BeforeEach(func() {
				releaseID = release.ID{Name: "uaa", Version: "123"}
				desiredRelease = release.Requirement{
					Name:            "uaa",
					StemcellVersion: "621.71",
				}

				fakeS3Client = new(fakes.S3Client)
				object1Key := "uaa/uaa-122.tgz"
				object2Key := "uaa/uaa-123.tgz"
				object3Key := "uaa/uaa-123.tgz"
				object4Key := "uaa/uaa-121.tgz"
				fakeS3Client.ListObjectsV2Returns(&s3.ListObjectsV2Output{
					Contents: []*s3.Object{
						{Key: &object1Key},
						{Key: &object3Key},
						{Key: &object2Key},
						{Key: &object4Key},
					},
				}, nil)

				logger = log.New(GinkgoWriter, "", 0)
				fakeS3Downloader := new(fakes.S3Downloader)
				// fakeS3Downloader writes the given S3 bucket and key into the output file for easy verification
				fakeS3Downloader.DownloadStub = func(writer io.WriterAt, objectInput *s3.GetObjectInput, setConcurrency ...func(dl *s3manager.Downloader)) (int64, error) {
					n, err := writer.WriteAt([]byte(fmt.Sprintf("%s/%s", *objectInput.Bucket, *objectInput.Key)), 0)
					return int64(n), err
				}

				releaseSource = NewS3ReleaseSource(
					sourceID,
					bucket,
					`{{.Name}}/{{.Name}}-{{.Version}}.tgz`,
					false,
					fakeS3Client,
					fakeS3Downloader,
					nil,
					logger,
				)
				uaaKey = "uaa/uaa-123.tgz"
			})

			It("gets the latest version of a release", func() {
				remoteRelease, found, err := releaseSource.FindReleaseVersion(desiredRelease)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(fakeS3Client.ListObjectsV2CallCount()).To(Equal(1))
				input := fakeS3Client.ListObjectsV2ArgsForCall(0)
				Expect(*input.Prefix).To(Equal("uaa/"))

				Expect(remoteRelease).To(Equal(release.Remote{
					ID:         releaseID,
					RemotePath: uaaKey,
					SourceID:   sourceID,
					SHA:        "bc7cb372ee4b9a9d6f4e8a993d46405d2c114e9c",
				}))
			})
		})
	})

	Describe("FindReleaseVersion from S3 compiled-releases", func() {
		const bucket = "compiled-releases"

		var (
			releaseSource  S3ReleaseSource
			fakeS3Client   *fakes.S3Client
			desiredRelease release.Requirement
			releaseID      release.ID
			uaaKey         string
			logger         *log.Logger
		)
		When("version is semantic and has 2 latest versions with different stemcell versions", func() {
			BeforeEach(func() {
				releaseID = release.ID{Name: "uaa", Version: "1.2.3"}
				desiredRelease = release.Requirement{
					Name:            "uaa",
					StemcellVersion: "621.71",
				}

				fakeS3Client = new(fakes.S3Client)
				object1Key := "2.11/uaa/uaa-1.2.2-ubuntu-xenial-621.71.tgz"
				object2Key := "2.11/uaa/uaa-1.2.3-ubuntu-xenial-621.71.tgz"
				object3Key := "2.11/uaa/uaa-1.2.1-ubuntu-xenial-621.71.tgz"
				object4Key := "2.11/uaa/uaa-1.2.3-ubuntu-xenial-622.71.tgz"
				fakeS3Client.ListObjectsV2Returns(&s3.ListObjectsV2Output{
					Contents: []*s3.Object{
						{Key: &object1Key},
						{Key: &object4Key},
						{Key: &object3Key},
						{Key: &object2Key},
					},
				}, nil)

				logger = log.New(GinkgoWriter, "", 0)
				fakeS3Downloader := new(fakes.S3Downloader)
				// fakeS3Downloader writes the given S3 bucket and key into the output file for easy verification
				fakeS3Downloader.DownloadStub = func(writer io.WriterAt, objectInput *s3.GetObjectInput, setConcurrency ...func(dl *s3manager.Downloader)) (int64, error) {
					n, err := writer.WriteAt([]byte(fmt.Sprintf("%s/%s", *objectInput.Bucket, *objectInput.Key)), 0)
					return int64(n), err
				}

				releaseSource = NewS3ReleaseSource(
					sourceID,
					bucket,
					`2.11/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
					true,
					fakeS3Client,
					fakeS3Downloader,
					nil,
					logger,
				)
				uaaKey = "2.11/uaa/uaa-1.2.3-ubuntu-xenial-621.71.tgz"
			})

			It("gets the latest version of a release", func() {
				remoteRelease, found, err := releaseSource.FindReleaseVersion(desiredRelease)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(fakeS3Client.ListObjectsV2CallCount()).To(Equal(1))
				input := fakeS3Client.ListObjectsV2ArgsForCall(0)
				Expect(*input.Prefix).To(Equal("2.11/uaa/"))

				Expect(remoteRelease).To(Equal(release.Remote{
					ID:         releaseID,
					RemotePath: uaaKey,
					SourceID:   sourceID,
					SHA:        "78facf87f730395fb263fb5e89157c438fc1d8a9",
				}))
			})
		})
	})

	Describe("UploadRelease", func() {
		var (
			s3Uploader    *fakes.S3Uploader
			releaseSource S3ReleaseSource
			file          io.Reader
		)

		BeforeEach(func() {
			s3Uploader = new(fakes.S3Uploader)
			releaseSource = NewS3ReleaseSource(
				sourceID,
				"orange-bucket",
				`{{.Name}}/{{.Name}}-{{.Version}}.tgz`,
				false,
				nil,
				nil,
				s3Uploader,
				log.New(GinkgoWriter, "", 0),
			)
			file = strings.NewReader("banana banana")
		})

		Context("happy path", func() {
			It("uploads the file to the correct location", func() {
				_, err := releaseSource.UploadRelease(release.Requirement{
					Name:    "banana",
					Version: "1.2.3",
				}, file)
				Expect(err).NotTo(HaveOccurred())

				Expect(s3Uploader.UploadCallCount()).To(Equal(1))

				opts, fns := s3Uploader.UploadArgsForCall(0)

				Expect(fns).To(HaveLen(0))

				Expect(opts.Bucket).To(PointTo(Equal("orange-bucket")))
				Expect(opts.Key).To(PointTo(Equal("banana/banana-1.2.3.tgz")))
				Expect(opts.Body).To(Equal(file))
			})

			It("returns the remote release", func() {
				remoteRelease, err := releaseSource.UploadRelease(release.Requirement{
					Name:    "banana",
					Version: "1.2.3",
				}, file)
				Expect(err).NotTo(HaveOccurred())

				Expect(remoteRelease).To(Equal(release.Remote{
					ID:         release.ID{Name: "banana", Version: "1.2.3"},
					RemotePath: "banana/banana-1.2.3.tgz",
					SourceID:   sourceID,
				}))
			})
		})

		When("there is an error evaluating the path template", func() {
			BeforeEach(func() {
				releaseSource = NewS3ReleaseSource(
					sourceID,
					"orange-bucket",
					`{{.NoSuchField}}`,
					false,
					nil,
					nil,
					s3Uploader,
					log.New(GinkgoWriter, "", 0),
				)
			})

			It("returns a descriptive error", func() {
				_, err := releaseSource.UploadRelease(release.Requirement{
					Name:    "banana",
					Version: "1.2.3",
				}, file)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
			})
		})
	})

	Describe("RemotePath", func() {
		var (
			releaseSource S3ReleaseSource
			requirement   release.Requirement
		)

		BeforeEach(func() {
			releaseSource = NewS3ReleaseSource(
				sourceID,
				"orange-bucket",
				`{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
				false,
				nil,
				nil,
				nil,
				log.New(GinkgoWriter, "", 0),
			)
			requirement = release.Requirement{
				Name:            "bob",
				Version:         "2.0",
				StemcellOS:      "plan9",
				StemcellVersion: "42",
			}
		})

		It("returns the remote path for the given requirement", func() {
			path, err := releaseSource.RemotePath(requirement)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal("bob/bob-2.0-plan9-42.tgz"))
		})

		When("there is an error evaluating the path template", func() {
			BeforeEach(func() {
				releaseSource = NewS3ReleaseSource(
					sourceID,
					"orange-bucket",
					`{{.NoSuchField}}`,
					false,
					nil,
					nil,
					nil,
					log.New(GinkgoWriter, "", 0),
				)
			})

			It("returns a descriptive error", func() {
				_, err := releaseSource.RemotePath(requirement)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
			})
		})
	})
})
