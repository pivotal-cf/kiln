package component

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"

	"github.com/google/go-github/v40/github"

	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component/fakes_internal"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestGithubReleaseSource_interface_compliance(t *testing.T) {
	t.Run("non-ptr", func(t *testing.T) {
		please := NewWithT(t)
		var grs any = GithubReleaseSource{}
		_, ok := grs.(ReleaseSource)
		please.Expect(ok).To(BeFalse())
	})
	t.Run("ptr", func(t *testing.T) {
		please := NewWithT(t)
		var grs any = new(GithubReleaseSource)
		_, ok := grs.(ReleaseSource)
		please.Expect(ok).To(BeTrue())
	})
}

func TestGithubReleaseSource_downloadRelease(t *testing.T) {
	lock := cargo.BOSHReleaseTarballLock{
		Name:       "routing",
		Version:    "0.239.0",
		RemotePath: "https://github.com/cloudfoundry/routing-release/releases/download/v0.239.0/routing-0.239.0.tgz",
	}

	please := NewWithT(t)
	tempDir := t.TempDir()
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	asset := bytes.NewBufferString("some contents\n")

	downloader := new(fakes_internal.ReleaseByTagGetterAssetDownloader)
	downloader.GetReleaseByTagReturnsOnCall(0, nil, nil, errors.New("banana"))
	downloader.GetReleaseByTagReturnsOnCall(1, &github.RepositoryRelease{
		Assets: []*github.ReleaseAsset{
			{
				Name: ptr("routing-0.239.0.tgz"),
			},
		},
	}, nil, nil)
	downloader.DownloadReleaseAssetReturns(io.NopCloser(asset), "", nil)

	logger := log.New(io.Discard, "", 0)
	local, err := downloadRelease(context.Background(), tempDir, lock, downloader, logger)
	please.Expect(err).NotTo(HaveOccurred())

	{
		_, org, repo, tag := downloader.GetReleaseByTagArgsForCall(0)
		please.Expect(org).To(Equal("cloudfoundry"))
		please.Expect(repo).To(Equal("routing-release"))
		please.Expect(tag).To(Equal("v0.239.0"))
	}
	{
		_, org, repo, tag := downloader.GetReleaseByTagArgsForCall(1)
		please.Expect(org).To(Equal("cloudfoundry"))
		please.Expect(repo).To(Equal("routing-release"))
		please.Expect(tag).To(Equal("0.239.0"))
	}

	please.Expect(local.LocalPath).To(BeAnExistingFile(), "it finds the created asset file")
	please.Expect(local.Lock.SHA1).To(Equal("3a2be7b07a1a19072bf54c95a8c4a3fe0cdb35d4"))
}

func ptr[T any](v T) *T { return &v }
