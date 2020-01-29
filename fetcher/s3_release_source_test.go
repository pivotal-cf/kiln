package fetcher_test

import (
	"errors"
	"fmt"
	. "github.com/onsi/gomega/gstruct"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	Describe("DownloadReleases", func() {
		const bucket = "some-bucket"

		var (
			logger           *log.Logger
			releaseSource    S3ReleaseSource
			releaseDir       string
			remoteRelease    release.Remote
			releaseID        release.ID
			fakeS3Downloader *fakes.S3Downloader
		)

		BeforeEach(func() {
			var err error

			releaseDir, err = ioutil.TempDir("", "kiln-releaseSource-test")
			Expect(err).NotTo(HaveOccurred())

			releaseID = release.ID{Name: "uaa", Version: "1.2.3"}
			remoteRelease = release.Remote{ID: releaseID, RemotePath: "some-uaa-key", SourceID: bucket}

			logger = log.New(GinkgoWriter, "", 0)
			fakeS3Downloader = new(fakes.S3Downloader)
			// fakeS3Downloader writes the given S3 bucket and key into the output file for easy verification
			fakeS3Downloader.DownloadStub = func(writer io.WriterAt, objectInput *s3.GetObjectInput, setConcurrency ...func(dl *s3manager.Downloader)) (int64, error) {
				n, err := writer.WriteAt([]byte(fmt.Sprintf("%s/%s", *objectInput.Bucket, *objectInput.Key)), 0)
				return int64(n), err
			}
			releaseSource = S3ReleaseSource{
				Logger:       logger,
				S3Downloader: fakeS3Downloader,
				Bucket:       bucket,
			}
		})

		AfterEach(func() {
			_ = os.RemoveAll(releaseDir)
		})

		It("downloads the appropriate versions of built releases listed in remoteReleases", func() {
			localRelease, err := releaseSource.DownloadRelease(releaseDir, remoteRelease, 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(1))

			releasePath := filepath.Join(releaseDir, "uaa-1.2.3.tgz")
			releaseContents, err := ioutil.ReadFile(releasePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(releaseContents).To(Equal([]byte("some-bucket/some-uaa-key")))

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
			releaseSource  *S3ReleaseSource
			fakeS3Client   *fakes.S3HeadObjecter
			desiredRelease release.Requirement
			bpmReleaseID   release.ID
			bpmKey         string
		)

		BeforeEach(func() {
			bpmReleaseID = release.ID{Name: "bpm-release", Version: "1.2.3"}
			desiredRelease = release.Requirement{
				Name:            "bpm-release",
				Version:         "1.2.3",
				StemcellOS:      "ubuntu-xenial",
				StemcellVersion: "190.0.0",
			}

			fakeS3Client = new(fakes.S3HeadObjecter)
			fakeS3Client.HeadObjectReturns(new(s3.HeadObjectOutput), nil)

			logger := log.New(nil, "", 0)

			releaseSource = &S3ReleaseSource{
				Logger:       logger,
				S3Client:     fakeS3Client,
				PathTemplate: `2.5/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
				Bucket:       bucket,
			}
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
				SourceID:   bucket,
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
				releaseSource.PathTemplate = "{{.NoSuchField}}"
			})

			It("returns a descriptive error", func() {
				_, found, err := releaseSource.GetMatchedRelease(desiredRelease)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("UploadRelease", func() {
		var (
			s3Uploader    *fakes.S3Uploader
			releaseSource *S3ReleaseSource
			file          io.Reader
		)

		BeforeEach(func() {
			s3Uploader = new(fakes.S3Uploader)
			releaseSource = &S3ReleaseSource{
				S3Uploader:   s3Uploader,
				Bucket:       "orange-bucket",
				Logger:       log.New(GinkgoWriter, "", 0),
				PathTemplate: `{{.Name}}/{{.Name}}-{{.Version}}.tgz`,
			}
			file = strings.NewReader("banana banana")
		})

		Context("happy path", func() {
			It("uploads the file to the correct location", func() {
				Expect(
					releaseSource.UploadRelease("banana", "1.2.3", file),
				).To(Succeed())

				Expect(s3Uploader.UploadCallCount()).To(Equal(1))

				opts, fns := s3Uploader.UploadArgsForCall(0)

				Expect(fns).To(HaveLen(0))

				Expect(opts.Bucket).To(PointTo(Equal("orange-bucket")))
				Expect(opts.Key).To(PointTo(Equal("banana/banana-1.2.3.tgz")))
				Expect(opts.Body).To(Equal(file))
			})
		})

		When("there is an error evaluating the path template", func() {
			BeforeEach(func() {
				releaseSource.PathTemplate = "{{.NoSuchField}}"
			})

			It("returns a descriptive error", func() {
				err := releaseSource.UploadRelease("banana", "1.2.3", file)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
			})
		})
	})

	Describe("RemotePath", func() {
		var (
			releaseSource *S3ReleaseSource
			requirement   release.Requirement
		)

		BeforeEach(func() {
			releaseSource = &S3ReleaseSource{
				S3Uploader:   nil,
				Bucket:       "orange-bucket",
				Logger:       log.New(GinkgoWriter, "", 0),
				PathTemplate: `{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
			}
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
				releaseSource.PathTemplate = "{{.NoSuchField}}"
			})

			It("returns a descriptive error", func() {
				_, err := releaseSource.RemotePath(requirement)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
			})
		})
	})
})
