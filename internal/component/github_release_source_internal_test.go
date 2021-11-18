package component

import (
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"os"
	"testing"
)

func TestGithubReleaseSource_downloadRelease(t *testing.T) {
	lock := Lock{Name: "routing", Version: "0.226.0"}

	t.Run("when the release is downloaded", func(t *testing.T) {
		damnIt := Ω.NewWithT(t)
		tempDir := t.TempDir()
		t.Cleanup(func() {
			_ = os.RemoveAll(tempDir)
		})

		ghClient := new(fakes.GithubNewRequestDoer)

		local, err := downloadRelease(tempDir, lock, ghClient)
		damnIt.Expect(err).NotTo(Ω.HaveOccurred())

		_, err = os.Stat(local.LocalPath)
		damnIt.Expect(err).NotTo(Ω.HaveOccurred(), "it creates the expected asset")
	})
	//grs.Client.Repositories.DownloadReleaseAsset(context.TODO(), testLock.)

	//Mocking up the Lock we'll need to test
	/*
		strPtr := func(s string) *string { return &s }
		intPtr := func(i int64) *int64 { return &i }
		releaseGetter := new(fakes.ReleaseByTagGetter)
		downloader := new(fakes.ReleaseAssetDownloader)

		releaseGetter.GetReleaseByTagReturns(
			&github.RepositoryRelease{
				TagName: strPtr("0.226.0"),
				Assets: []*github.ReleaseAsset{
					{
						Name:               strPtr("routing-0.226.0.tgz.sha256"),
						BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz.sha256"),
					},
					{
						Name:               strPtr("routing-0.226.0.tgz"),
						BrowserDownloadURL: strPtr("https://github.com/cloudfoundry/routing-release/releases/download/0.226.0/routing-0.226.0.tgz"),
						ID:                 intPtr(420),
					},
				},
			},
			&github.Response{Response: &http.Response{StatusCode: http.StatusOK}},
			nil,
		)

		file := &SetTrueOnClose{Reader: bytes.NewBufferString("hello")}
		downloader.DownloadReleaseAssetReturns(file, "", nil)

		ctx := context.TODO()

		testReleaseLock, _, _ := component.LockFromGithubRelease(ctx, downloader, "cloudfoundry", component.Spec{
			Name:    "routing",
			Version: "0.226.0",
			GitRepositories: []string{
				"https://github.com/cloudfoundry/routing-release",
			},
		}, component.GetGithubReleaseWithTag(releaseGetter, "0.226.0"))
	*/
}
