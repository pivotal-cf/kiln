package component

import (
	"bytes"
	"context"
	"errors"
	"github.com/pivotal-cf/kiln/internal/component/fakes_internal"
	"io"
	"log"
	"os"
	"testing"

	"github.com/google/go-github/v40/github"

	Ω "github.com/onsi/gomega"
)

func TestGithubReleaseSource_downloadRelease(t *testing.T) {
	lock := Lock{Name: "routing", Version: "0.226.0", RemotePath: "https://github.com/cloudfoundry/routing-release/"}

	please := Ω.NewWithT(t)
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
				Name: ptr("routing-0.226.0.tgz"),
			},
		},
	}, nil, nil)
	downloader.DownloadReleaseAssetReturns(io.NopCloser(asset), "", nil)

	logger := log.New(io.Discard, "", 0)
	local, err := downloadRelease(context.Background(), tempDir, lock, downloader, logger)
	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(local.LocalPath).To(Ω.BeAnExistingFile(), "it finds the created asset file")
	please.Expect(local.SHA1).To(Ω.Equal("3a2be7b07a1a19072bf54c95a8c4a3fe0cdb35d4"))
}
