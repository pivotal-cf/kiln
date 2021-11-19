package component

import (
	"context"
	"github.com/google/go-github/v40/github"
	Ω "github.com/onsi/gomega"
	fakes "github.com/pivotal-cf/kiln/internal/component/fakes_internal"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
)

func TestGithubReleaseSource_downloadRelease(t *testing.T) {
	lock := Lock{Name: "routing", Version: "0.226.0"}

	damnIt := Ω.NewWithT(t)
	tempDir := t.TempDir()
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	ghClient := new(fakes.GithubNewRequestDoer)
	ghClient.NewRequestReturns(&http.Request{}, nil)

	ghClient.DoStub = func(_ context.Context, _ *http.Request, i interface{}) (*github.Response, error) {
		w, ok := i.(io.Writer)
		if !ok {
			t.Error("expected a writer")
		}
		_, _ = w.Write([]byte("hello"))
		return &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil
	}

	logger := log.New(io.Discard, "", 0)
	local, err := downloadRelease(context.Background(), tempDir, lock, ghClient, logger)
	damnIt.Expect(err).NotTo(Ω.HaveOccurred())

	damnIt.Expect(local.LocalPath).To(Ω.BeAnExistingFile(), "it finds the created asset file")
	damnIt.Expect(local.SHA1).To(Ω.Equal("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"))
}
