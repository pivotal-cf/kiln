package fetcher_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	providerfakes "github.com/pivotal-cf/kiln/internal/providers/fakes"
)

var _ = Describe("Downloader", func() {
	var (
		logger           *log.Logger
		downloader       fetcher.Downloader
		compiledReleases cargo.CompiledReleases
		releaseDir       string
		matchedS3Objects map[cargo.CompiledRelease]string
		fakeS3Downloader *providerfakes.S3Downloader
	)

	BeforeEach(func() {
		var err error

		compiledReleases = cargo.CompiledReleases{
			Bucket: "some-bucket",
		}

		releaseDir, err = ioutil.TempDir("", "kiln-downloader-test")
		Expect(err).NotTo(HaveOccurred())

		matchedS3Objects = make(map[cargo.CompiledRelease]string)
		matchedS3Objects[cargo.CompiledRelease{Name: "uaa", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-uaa-key"
		matchedS3Objects[cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-bpm-key"

		logger = log.New(GinkgoWriter, "", 0)
		fakeS3Downloader = new(providerfakes.S3Downloader)
		// fakeS3Downloader writes the given S3 bucket and key into the output file for easy verification
		fakeS3Downloader.DownloadStub = func(writer io.WriterAt, objectInput *s3.GetObjectInput, setConcurrency ...func(dl *s3manager.Downloader)) (int64, error) {
			n, err := writer.WriteAt([]byte(fmt.Sprintf("%s/%s", *objectInput.Bucket, *objectInput.Key)), 0)
			return int64(n), err
		}
		fakeS3Provider := new(fakes.S3Provider)
		fakeS3Provider.GetS3DownloaderReturns(fakeS3Downloader)
		downloader = fetcher.NewDownloader(logger, fakeS3Provider)
	})

	AfterEach(func() {
		_ = os.RemoveAll(releaseDir)
	})

	It("downloads the appropriate versions of releases listed in matchedS3Objects", func() {
		err := downloader.DownloadReleases(releaseDir, compiledReleases, matchedS3Objects, 7)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(2))

		bpmContents, err := ioutil.ReadFile(filepath.Join(releaseDir, "bpm-1.2.3-ubuntu-trusty-1234.tgz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(bpmContents).To(Equal([]byte("some-bucket/some-bpm-key")))
		uaaContents, err := ioutil.ReadFile(filepath.Join(releaseDir, "uaa-1.2.3-ubuntu-trusty-1234.tgz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(uaaContents).To(Equal([]byte("some-bucket/some-uaa-key")))

		_, _, opts := fakeS3Downloader.DownloadArgsForCall(0)
		verifySetsConcurrency(opts, 7)

		_, _, opts = fakeS3Downloader.DownloadArgsForCall(1)
		verifySetsConcurrency(opts, 7)
	})

	Context("when number of threads is not specified", func() {
		It("uses the s3manager package's default download concurrency", func() {
			err := downloader.DownloadReleases(releaseDir, compiledReleases, matchedS3Objects, 0)
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
				err := downloader.DownloadReleases("/non-existent-folder", compiledReleases, matchedS3Objects, 0)
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
				err := downloader.DownloadReleases(releaseDir, compiledReleases, matchedS3Objects, 0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("failed to download file, 503 Service Unavailable\n"))
			})
		})
	})
})

func verifySetsConcurrency(opts []func(*s3manager.Downloader), concurrency int) {
	Expect(opts).To(HaveLen(1))

	downloader := &s3manager.Downloader{
		Concurrency: 1,
	}

	opts[0](downloader)

	Expect(downloader.Concurrency).To(Equal(concurrency))
}
