package fetcher_test

import (
	"errors"
	"fmt"
	. "github.com/onsi/gomega/gstruct"
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
			logger                     *log.Logger
			releaseSource              S3ReleaseSource
			releaseDir                 string
			remoteReleases             []release.RemoteRelease
			uaaReleaseID, bpmReleaseID release.ReleaseID
			fakeS3Downloader           *fakes.S3Downloader
		)

		BeforeEach(func() {
			var err error

			releaseDir, err = ioutil.TempDir("", "kiln-releaseSource-test")
			Expect(err).NotTo(HaveOccurred())

			uaaReleaseID = release.ReleaseID{Name: "uaa", Version: "1.2.3"}
			bpmReleaseID = release.ReleaseID{Name: "bpm", Version: "1.2.3"}
			remoteReleases = []release.RemoteRelease{
				{ReleaseID: uaaReleaseID, RemotePath: "some-uaa-key"},
				{ReleaseID: bpmReleaseID, RemotePath: "some-bpm-key"},
			}

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
			localReleases, err := releaseSource.DownloadReleases(releaseDir, remoteReleases, 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(2))

			bpmReleasePath := filepath.Join(releaseDir, "bpm-1.2.3.tgz")
			bpmContents, err := ioutil.ReadFile(bpmReleasePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(bpmContents).To(Equal([]byte("some-bucket/some-bpm-key")))
			uaaReleasePath := filepath.Join(releaseDir, "uaa-1.2.3.tgz")
			uaaContents, err := ioutil.ReadFile(uaaReleasePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(uaaContents).To(Equal([]byte("some-bucket/some-uaa-key")))

			_, _, opts := fakeS3Downloader.DownloadArgsForCall(0)
			verifySetsConcurrency(opts, 7)

			_, _, opts = fakeS3Downloader.DownloadArgsForCall(1)
			verifySetsConcurrency(opts, 7)

			Expect(localReleases).To(HaveLen(2))
			Expect(localReleases).To(ConsistOf(
				release.LocalRelease{ReleaseID: uaaReleaseID, LocalPath: uaaReleasePath},
				release.LocalRelease{ReleaseID: bpmReleaseID, LocalPath: bpmReleasePath},
			))
		})

		Context("when the remoteReleases argument is empty", func() {
			It("does not download anything from S3", func() {
				_, err := releaseSource.DownloadReleases(releaseDir, nil, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(0))
			})
		})

		Context("when number of threads is not specified", func() {
			It("uses the s3manager package's default download concurrency", func() {
				_, err := releaseSource.DownloadReleases(releaseDir, remoteReleases, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(2))

				_, _, opts1 := fakeS3Downloader.DownloadArgsForCall(0)
				verifySetsConcurrency(opts1, s3manager.DefaultDownloadConcurrency)

				_, _, opts2 := fakeS3Downloader.DownloadArgsForCall(1)
				verifySetsConcurrency(opts2, s3manager.DefaultDownloadConcurrency)
			})
		})

		Context("failure cases", func() {
			Context("when a file can't be created", func() {
				It("returns an error", func() {
					_, err := releaseSource.DownloadReleases("/non-existent-folder", remoteReleases, 0)
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
					_, err := releaseSource.DownloadReleases(releaseDir, remoteReleases, 0)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("failed to download file: 503 Service Unavailable\n"))
				})
			})
		})
	})

	Describe("GetMatchedReleases", func() {
		const bucket = "built-bucket"

		var (
			releaseSource     *S3ReleaseSource
			fakeS3Client      *fakes.S3HeadObjecter
			desiredReleaseSet release.ReleaseRequirementSet
			bpmReleaseID      release.ReleaseID
			bpmKey            string
		)

		BeforeEach(func() {
			bpmReleaseID = release.ReleaseID{Name: "bpm-release", Version: "1.2.3"}
			desiredReleaseSet = release.ReleaseRequirementSet{
				bpmReleaseID: release.ReleaseRequirement{
					Name:            bpmReleaseID.Name,
					Version:         bpmReleaseID.Version,
					StemcellOS:      "ubuntu-xenial",
					StemcellVersion: "190.0.0",
				},
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
			remoteReleases, err := releaseSource.GetMatchedReleases(desiredReleaseSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeS3Client.HeadObjectCallCount()).To(Equal(1))
			input := fakeS3Client.HeadObjectArgsForCall(0)
			Expect(input.Bucket).To(PointTo(BeEquivalentTo(bucket)))
			Expect(input.Key).To(PointTo(BeEquivalentTo(bpmKey)))

			Expect(remoteReleases).To(HaveLen(1))
			Expect(remoteReleases).To(ConsistOf(release.RemoteRelease{
				ReleaseID:  bpmReleaseID,
				RemotePath: bpmKey,
			}))
		})

		When("the requested releases doesn't exist in the bucket", func() {
			BeforeEach(func() {
				notFoundError := new(fakes.S3RequestFailure)
				notFoundError.StatusCodeReturns(404)
				fakeS3Client.HeadObjectReturns(nil, notFoundError)
			})

			It("does not return them", func() {
				remoteReleases, err := releaseSource.GetMatchedReleases(desiredReleaseSet)
				Expect(err).NotTo(HaveOccurred())

				Expect(remoteReleases).To(HaveLen(0))
			})
		})

		When("there is an error evaluating the path template", func() {
			BeforeEach(func() {
				releaseSource.PathTemplate = "{{.NoSuchField}}"
			})

			It("returns a descriptive error", func() {
				_, err := releaseSource.GetMatchedReleases(desiredReleaseSet)

				Expect(err).To(MatchError(ContainSubstring(`unable to evaluate path_template`)))
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
})
