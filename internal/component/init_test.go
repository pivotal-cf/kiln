package component

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGitHubReleaseSource_init(t *testing.T) {
	source := GitHubReleaseSource{
		GithubToken: "banana",
	}

	err := source.init(context.Background())

	please := NewWithT(t)
	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(source.collaborators.client).NotTo(BeNil())
}

func TestS3ReleaseSource_init(t *testing.T) {
	source := S3ReleaseSource{}

	err := source.init()

	please := NewWithT(t)
	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(source.Collaborators.S3Client).NotTo(BeNil())
	please.Expect(source.Collaborators.S3Downloader).NotTo(BeNil())
	please.Expect(source.Collaborators.S3Uploader).NotTo(BeNil())
}
