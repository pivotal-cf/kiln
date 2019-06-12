package fetcher_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	providerfakes "github.com/pivotal-cf/kiln/internal/providers/fakes"
)

func verifySetsConcurrency(opts []func(*s3manager.Downloader), concurrency int) {
	Expect(opts).To(HaveLen(1))

	downloader := &s3manager.Downloader{
		Concurrency: 1,
	}

	opts[0](downloader)

	Expect(downloader.Concurrency).To(Equal(concurrency))
}

var _ = Describe("S3Connecter.Connect()", func() {
	var fakeS3Provider *fakes.S3Provider

	It("returns a new S3ReleaseSource", func() {
		fakeS3Provider = new(fakes.S3Provider)
		logger := log.New(nil, "", 0)
		s3Connecter := fetcher.NewS3Connecter(fakeS3Provider, logger)

		//Here we should use the s3Client Fake instead of making a
		//s3Client and s3Downloader with the Connect function.
		//However, I think that means that we would have to
		//write a new Connect function that takes in the fake client
		//and fake downloader(?). But the purpose of this test was to
		//test the connect on an s3Connecter... and if we fake it out
		//are we really testing what we want to test.
		compiledReleases := cargo.CompiledReleases{
			Bucket:          "some-bucket",
			Region:          "north-east-1",
			AccessKeyId:     "newkey",
			SecretAccessKey: "newsecret",
			Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+(-\w+(\.[0-9]+)?)?)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
		}
		s3ReleaseSource := s3Connecter.Connect(compiledReleases)
		Expect(s3ReleaseSource).ToNot(BeNil())
	})

	It("lists all objects that match the given regex", func() {
		var (
			releaseSource    commands.ReleaseSource
			fakeS3Provider   *fakes.S3Provider
			fakeS3Client     *fakes.S3Client
			compiledReleases cargo.CompiledReleases
			assetsLock       cargo.AssetsLock
			bpmKey           string
		)
		assetsLock = cargo.AssetsLock{
			Releases: []cargo.Release{
				{Name: "bpm", Version: "1.2.3-lts"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "ubuntu-xenial",
				Version: "190.0.0",
			},
		}

		fakeS3Client = new(fakes.S3Client)

		irrelevantKey := "some-key"
		uaaKey := "1.10/uaa/uaa-1.2.3-ubuntu-xenial-190.0.0.tgz"
		bpmKey = "2.5/bpm/bpm-1.2.3-lts-ubuntu-xenial-190.0.0.tgz"
		fakeS3Client.ListObjectsPagesStub = func(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
			shouldContinue := fn(&s3.ListObjectsOutput{
				Contents: []*s3.Object{
					{Key: &irrelevantKey},
					{Key: &uaaKey},
					{Key: &bpmKey},
				},
			},
				true,
			)
			Expect(shouldContinue).To(BeTrue())
			return nil
		}

		fakeS3Provider = new(fakes.S3Provider)
		fakeS3Provider.GetS3ClientReturns(fakeS3Client)

		logger := log.New(nil, "", 0)
		s3Connecter := fetcher.NewS3Connecter(fakeS3Provider, logger)

		compiledReleases = cargo.CompiledReleases{
			Bucket:          "some-bucket",
			Region:          "north-east-1",
			AccessKeyId:     "newkey",
			SecretAccessKey: "newsecret",
			Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+(-\w+(\.[0-9]+)?)?)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
		}

		releaseSource = s3Connecter.Connect(compiledReleases)

		matchedS3Objects, _, err := releaseSource.GetMatchedReleases(compiledReleases, assetsLock)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeS3Provider.GetS3ClientCallCount()).To(Equal(1))
		region, accessKeyId, secretAccessKey := fakeS3Provider.GetS3ClientArgsForCall(0)
		Expect(region).To(Equal("north-east-1"))
		Expect(accessKeyId).To(Equal("newkey"))
		Expect(secretAccessKey).To(Equal("newsecret"))

		input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
		Expect(input.Bucket).To(Equal(aws.String("some-bucket")))

		Expect(matchedS3Objects).To(HaveLen(1))
		Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3-lts", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, bpmKey))
	})
})

var _ = Describe("GetMatchedReleases from S3", func() {
	var (
		releaseSource    commands.ReleaseSource
		fakeS3Provider   *fakes.S3Provider
		fakeS3Client     *fakes.S3Client
		compiledReleases cargo.CompiledReleases
		assetsLock       cargo.AssetsLock
		bpmKey           string
	)

	BeforeEach(func() {
		assetsLock = cargo.AssetsLock{
			Releases: []cargo.Release{
				{Name: "bpm", Version: "1.2.3-lts"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "ubuntu-xenial",
				Version: "190.0.0",
			},
		}

		fakeS3Client = new(fakes.S3Client)

		irrelevantKey := "some-key"
		uaaKey := "1.10/uaa/uaa-1.2.3-ubuntu-xenial-190.0.0.tgz"
		bpmKey = "2.5/bpm/bpm-1.2.3-lts-ubuntu-xenial-190.0.0.tgz"
		fakeS3Client.ListObjectsPagesStub = func(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
			shouldContinue := fn(&s3.ListObjectsOutput{
				Contents: []*s3.Object{
					{Key: &irrelevantKey},
					{Key: &uaaKey},
					{Key: &bpmKey},
				},
			},
				true,
			)
			Expect(shouldContinue).To(BeTrue())
			return nil
		}

		fakeS3Provider = new(fakes.S3Provider)
		fakeS3Provider.GetS3ClientReturns(fakeS3Client)

		logger := log.New(nil, "", 0)
		s3Connecter := fetcher.NewS3Connecter(fakeS3Provider, logger)

		compiledReleases = cargo.CompiledReleases{
			Bucket:          "some-bucket",
			Region:          "north-east-1",
			AccessKeyId:     "newkey",
			SecretAccessKey: "newsecret",
			Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+(-\w+(\.[0-9]+)?)?)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
		}

		releaseSource = s3Connecter.Connect(compiledReleases)
	})

	It("lists all objects that match the given regex", func() {
		matchedS3Objects, _, err := releaseSource.GetMatchedReleases(compiledReleases, assetsLock)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeS3Provider.GetS3ClientCallCount()).To(Equal(1))
		region, accessKeyId, secretAccessKey := fakeS3Provider.GetS3ClientArgsForCall(0)
		Expect(region).To(Equal("north-east-1"))
		Expect(accessKeyId).To(Equal("newkey"))
		Expect(secretAccessKey).To(Equal("newsecret"))

		input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
		Expect(input.Bucket).To(Equal(aws.String("some-bucket")))

		Expect(matchedS3Objects).To(HaveLen(1))
		Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3-lts", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, bpmKey))
	})

	Context("if any objects in S3 do not match a release specified in assets.lock", func() {
		BeforeEach(func() {
			wrongReleaseVersionKey := "2.5/bpm/bpm-4.5.6-ubuntu-xenial-190.0.0.tgz"
			wrongReleaseNameKey := "2.5/diego/diego-1.2.3-ubuntu-xenial-190.0.0.tgz"
			fakeS3Client.ListObjectsPagesStub = func(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
				shouldContinue := fn(&s3.ListObjectsOutput{
					Contents: []*s3.Object{
						{Key: &wrongReleaseVersionKey},
						{Key: &wrongReleaseNameKey},
						{Key: &bpmKey},
					},
				},
					true,
				)
				Expect(shouldContinue).To(BeTrue())
				return nil
			}
		})

		It("does not return them", func() {
			matchedS3Objects, _, err := releaseSource.GetMatchedReleases(compiledReleases, assetsLock)
			Expect(err).NotTo(HaveOccurred())

			Expect(matchedS3Objects).To(HaveLen(1))
			Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3-lts", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, bpmKey))
		})
	})

	Context("if any objects in S3 do not match the provided stemcell criterion", func() {
		BeforeEach(func() {
			wrongStemcellVersionKey := "2.5/capi/capi-1.2.3-ubuntu-xenial-190.30.0.tgz"
			wrongStemcellOSKey := "2.5/diego/diego-1.2.3-windows-1803.0.0.tgz"
			fakeS3Client.ListObjectsPagesStub = func(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
				shouldContinue := fn(&s3.ListObjectsOutput{
					Contents: []*s3.Object{
						{Key: &wrongStemcellVersionKey},
						{Key: &wrongStemcellOSKey},
						{Key: &bpmKey},
					},
				},
					true,
				)
				Expect(shouldContinue).To(BeTrue())
				return nil
			}
		})

		It("does not return them", func() {
			matchedS3Objects, _, err := releaseSource.GetMatchedReleases(compiledReleases, assetsLock)
			Expect(err).NotTo(HaveOccurred())

			Expect(matchedS3Objects).To(HaveLen(1))
			Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3-lts", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, bpmKey))
		})
	})

	Context("if any objects in assets.lock don't have matches in S3,", func() {
		BeforeEach(func() {
			assetsLock.Releases = []cargo.Release{
				{Name: "bpm", Version: "1.2.3-lts"},
				{Name: "some-release", Version: "1.2.3"},
				{Name: "another-missing-release", Version: "4.5.6"},
			}
		})

		It("the missing objects return in `missingReleases`", func() {
			_, missingReleases, err := releaseSource.GetMatchedReleases(compiledReleases, assetsLock)

			Expect(err).ToNot(HaveOccurred())

			someReleaseMissingRelease := cargo.CompiledRelease{
				Name:            "some-release",
				Version:         "1.2.3",
				StemcellOS:      "ubuntu-xenial",
				StemcellVersion: "190.0.0",
			}
			anotherReleaseMissingRelease := cargo.CompiledRelease{
				Name:            "another-missing-release",
				Version:         "4.5.6",
				StemcellOS:      "ubuntu-xenial",
				StemcellVersion: "190.0.0",
			}
			Expect(len(missingReleases)).To(Equal(2))
			Expect(missingReleases).Should(ConsistOf(someReleaseMissingRelease, anotherReleaseMissingRelease))

		})

	})
})

var _ = Describe("S3ReleaseSource DownloadReleases", func() {
	var (
		logger           *log.Logger
		releaseSource    commands.ReleaseSource
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

		releaseDir, err = ioutil.TempDir("", "kiln-releaseSource-test")
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
		releaseSource = fetcher.NewS3Connecter(fakeS3Provider, logger).Connect(compiledReleases)

	})

	AfterEach(func() {
		_ = os.RemoveAll(releaseDir)
	})

	It("downloads the appropriate versions of releases listed in matchedS3Objects", func() {
		err := releaseSource.DownloadReleases(releaseDir, compiledReleases, matchedS3Objects, 7)
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

	Context("when the matchedS3Objects argument is empty", func() {
		It("does not download anything from S3", func() {
			err := releaseSource.DownloadReleases(releaseDir, compiledReleases, map[cargo.CompiledRelease]string{}, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeS3Downloader.DownloadCallCount()).To(Equal(0))
		})
	})

	Context("when number of threads is not specified", func() {
		It("uses the s3manager package's default download concurrency", func() {
			err := releaseSource.DownloadReleases(releaseDir, compiledReleases, matchedS3Objects, 0)
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
				err := releaseSource.DownloadReleases("/non-existent-folder", compiledReleases, matchedS3Objects, 0)
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
				err := releaseSource.DownloadReleases(releaseDir, compiledReleases, matchedS3Objects, 0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("failed to download file, 503 Service Unavailable\n"))
			})
		})
	})
})
