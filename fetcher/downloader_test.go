package fetcher_test

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
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
		fileCreator      func(string) (io.WriterAt, error)
		compiledReleases cargo.CompiledReleases
		fakeBPMFile      *os.File
		fakeUAAFile      *os.File
		matchedS3Objects map[cargo.CompiledRelease]string
		bpmInput         *s3.GetObjectInput
		uaaInput         *s3.GetObjectInput
		fakeS3Downloader *providerfakes.S3Downloader
	)

	BeforeEach(func() {
		var err error

		compiledReleases = cargo.CompiledReleases{
			Bucket: "some-bucket",
		}

		matchedS3Objects = make(map[cargo.CompiledRelease]string)
		matchedS3Objects[cargo.CompiledRelease{Name: "uaa", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-uaa-key"
		matchedS3Objects[cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-bpm-key"

		fakeBPMFile, err = ioutil.TempFile("", "bpm-release")
		Expect(err).NotTo(HaveOccurred())
		fakeUAAFile, err = ioutil.TempFile("", "uaa-release")
		Expect(err).NotTo(HaveOccurred())

		bpmInput = &s3.GetObjectInput{Bucket: aws.String("some-bucket"), Key: aws.String("some-bpm-key")}
		uaaInput = &s3.GetObjectInput{Bucket: aws.String("some-bucket"), Key: aws.String("some-uaa-key")}

		fileCreator = func(filepath string) (io.WriterAt, error) {
			if filepath == "releases/uaa-1.2.3-ubuntu-trusty-1234.tgz" {
				return fakeUAAFile, nil
			} else if filepath == "releases/bpm-1.2.3-ubuntu-trusty-1234.tgz" {
				return fakeBPMFile, nil
			}

			return nil, errors.New("unknown filepath")
		}

		logger = log.New(GinkgoWriter, "", 0)
		fakeS3Downloader = new(providerfakes.S3Downloader)
		fakeS3Provider := new(fakes.S3Provider)
		fakeS3Provider.GetS3DownloaderReturns(fakeS3Downloader)
		downloader = fetcher.NewDownloader(logger, fakeS3Provider, fileCreator)
	})

	AfterEach(func() {
		Expect(os.Remove(fakeBPMFile.Name())).To(Succeed())
		Expect(os.Remove(fakeUAAFile.Name())).To(Succeed())
	})

	It("downloads the appropriate versions of releases listed in matchedS3Objects", func() {
		err := downloader.DownloadReleases("releases", compiledReleases, matchedS3Objects, 7)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(2))

		w1, input1, opts := fakeS3Downloader.DownloadArgsForCall(0)
		verifySetsConcurrency(opts, 7)

		w2, input2, opts := fakeS3Downloader.DownloadArgsForCall(1)
		verifySetsConcurrency(opts, 7)

		inputs := map[io.WriterAt]*s3.GetObjectInput{
			w1: input1,
			w2: input2,
		}

		Expect(inputs).To(HaveKeyWithValue(fakeUAAFile, uaaInput))
		Expect(inputs).To(HaveKeyWithValue(fakeBPMFile, bpmInput))
	})

	Context("when number of threads is not specified", func() {
		It("uses the s3manager package's default download concurrency", func() {
			err := downloader.DownloadReleases("releases", compiledReleases, matchedS3Objects, 0)
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
			BeforeEach(func() {
				matchedS3Objects[cargo.CompiledRelease{Name: "not-real", Version: "-666", StemcellOS: "ubuntu-trusty", StemcellVersion: "1234"}] = "some-bpm-key"
			})

			It("returns an error", func() {
				err := downloader.DownloadReleases("releases", compiledReleases, matchedS3Objects, 0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("failed to create file \"not-real--666-ubuntu-trusty-1234.tgz\", unknown filepath"))
			})
		})

		Context("when a file can't be downloaded", func() {
			BeforeEach(func() {
				fakeS3Downloader.DownloadCalls(func(w io.WriterAt, i *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (int64, error) {
					return 0, errors.New("503 Service Unavailable")
				})
			})

			It("returns an error", func() {
				err := downloader.DownloadReleases("releases", compiledReleases, matchedS3Objects, 0)
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
