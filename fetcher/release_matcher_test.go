package fetcher_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("GetMatchedReleases", func() {
	var (
		releaseMatcher   fetcher.ReleaseMatcher
		fakeS3Provider   *fakes.S3Provider
		fakeS3Client     *fakes.S3Client
		compiledReleases cargo.CompiledReleases
		assetsLock       cargo.AssetsLock
		bpmKey           string
		err              error
	)

	BeforeEach(func() {
		assetsLock = cargo.AssetsLock{
			Releases: []cargo.Release{
				{Name: "bpm", Version: "1.2.3"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "ubuntu-xenial",
				Version: "190.0.0",
			},
		}

		compiledReleases = cargo.CompiledReleases{
			Bucket:          "some-bucket",
			Region:          "north-east-1",
			AccessKeyId:     "newkey",
			SecretAccessKey: "newsecret",
			Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
		}
		Expect(err).NotTo(HaveOccurred())
		fakeS3Client = new(fakes.S3Client)

		irrelevantKey := "some-key"
		uaaKey := "1.10/uaa/uaa-1.2.3-ubuntu-xenial-190.0.0.tgz"
		bpmKey = "2.5/bpm/bpm-1.2.3-ubuntu-xenial-190.0.0.tgz"
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

		releaseMatcher = fetcher.NewReleaseMatcher(fakeS3Provider)
	})

	It("lists all objects that match the given regex", func() {
		matchedS3Objects, err := releaseMatcher.GetMatchedReleases(compiledReleases, assetsLock)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeS3Provider.GetS3ClientCallCount()).To(Equal(1))
		region, accessKeyId, secretAccessKey := fakeS3Provider.GetS3ClientArgsForCall(0)
		Expect(region).To(Equal("north-east-1"))
		Expect(accessKeyId).To(Equal("newkey"))
		Expect(secretAccessKey).To(Equal("newsecret"))

		input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
		Expect(input.Bucket).To(Equal(aws.String("some-bucket")))

		Expect(matchedS3Objects).To(HaveLen(1))
		Expect(matchedS3Objects).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "bpm", Version: "1.2.3", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, bpmKey))
	})

	Context("if any objects in assets.lock don't have matches in S3", func() {
		BeforeEach(func() {
			assetsLock.Releases = []cargo.Release{
				{Name: "bpm", Version: "1.2.3"},
				{Name: "some-release", Version: "1.2.3"},
				{Name: "another-missing-release", Version: "4.5.6"},
			}
		})

		It("returns an error", func() {
			_, err = releaseMatcher.GetMatchedReleases(compiledReleases, assetsLock)
			Expect(err).To(MatchError(`Expected releases were not matched by the regex:
{Name:some-release Version:1.2.3 StemcellOS:ubuntu-xenial StemcellVersion:190.0.0}
{Name:another-missing-release Version:4.5.6 StemcellOS:ubuntu-xenial StemcellVersion:190.0.0}`))

			input, _ := fakeS3Client.ListObjectsPagesArgsForCall(0)
			Expect(input.Bucket).To(Equal(aws.String("some-bucket")))
		})
	})
})
