package commands_test

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("Fetch", func() {
	var (
		fetch commands.Fetch
	)

	Describe("ListObjects", func() {
		var (
			bucket       string
			regex        *regexp.Regexp
			fakeS3Client *fakes.S3Client
			err          error
		)

		BeforeEach(func() {
			bucket = "some-bucket"
			regex, err = regexp.Compile(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
			Expect(err).NotTo(HaveOccurred())

			fakeS3Client = new(fakes.S3Client)
		})

		It("lists all objects that match the given regex", func() {
			key1 := "some-key"
			key2 := "1.10/uaa/uaa-1.2.3-190.0.0.tgz"
			key3 := "2.5/bpm/bpm-1.2.3-190.0.0.tgz"
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

			matchedS3Objects, err := commands.ListObjects(bucket, regex, fakeS3Client)
			Expect(err).NotTo(HaveOccurred())

			input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
			Expect(input.Bucket).To(Equal(aws.String("some-bucket")))

			Expect(matchedS3Objects).To(HaveLen(1))
			Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellVersion: "190.0.0"}, key3))
		})
	})

	Describe("DownloadReleases", func() {
		var (
			assetsLock       cargo.AssetsLock
			bucket           string
			releasesDir      string
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
			releasesDir = "releases"

			matchedS3Objects = make(map[cargo.CompiledRelease]string)
			matchedS3Objects[cargo.CompiledRelease{Name: "uaa", Version: "1.2.3", StemcellVersion: "1234"}] = "some-uaa-key"
			matchedS3Objects[cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellVersion: "1234"}] = "some-bpm-key"

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
			err = commands.DownloadReleases(assetsLock, bucket, releasesDir, matchedS3Objects, fileCreator, fakeDownloader)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeDownloader.DownloadCallCount()).To(Equal(2))

			w1, input1, _ := fakeDownloader.DownloadArgsForCall(0)
			Expect(w1).To(Equal(fakeUAAFile))
			Expect(input1).To(Equal(uaaInput))

			w2, input2, _ := fakeDownloader.DownloadArgsForCall(1)
			Expect(w2).To(Equal(fakeBPMFile))
			Expect(input2).To(Equal(bpmInput))
		})

		It("returns an error if the release does not exist", func() {
			assetsLock.Releases = []cargo.Release{
				{Name: "not-real", Version: "1.2.3"},
			}

			err = commands.DownloadReleases(assetsLock, bucket, releasesDir, matchedS3Objects, fileCreator, fakeDownloader)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("Compiled release: not-real, version: 1.2.3, stemcell OS: ubuntu-trusty, stemcell version: 1234, not found"))
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			Expect(fetch.Usage()).To(Equal(jhanda.Usage{
				Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
				ShortDescription: "fetches releases",
				Flags:            fetch,
			}))
		})
	})
})
