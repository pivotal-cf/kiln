package scenario

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_githubRepoHasReleaseWithTag(t *testing.T) {
	t.Skip("skipping this test because it requires a github token to run. ")
	if isRunningInCI() {
		t.Skip("skip this step in CI. GitHub action credentials do not have access to crhntr/hello-release")
	}
	setup := func(t *testing.T) (context.Context, Gomega) {
		ctx := context.Background()

		dir, err := copyTileDirectory(t.TempDir(), filepath.Join("..", "testdata", "tiles", "v1"))
		if err != nil {
			t.Fatal(err)
		}

		ctx = setTileRepoPath(ctx, dir)
		ctx, err = loadGithubToken(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return ctx, NewWithT(t)
	}

	t.Run("release exists", func(t *testing.T) {
		ctx, please := setup(t)
		err := githubRepoHasReleaseWithTag(ctx, "crhntr", "hello-release", "v0.1.5")
		please.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("release does not exist", func(t *testing.T) {
		ctx, please := setup(t)
		err := githubRepoHasReleaseWithTag(ctx, "crhntr", "hello-release", "v99.99.99-banana")
		please.Expect(err).To(HaveOccurred())
	})
}
