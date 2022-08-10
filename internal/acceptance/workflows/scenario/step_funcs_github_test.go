package scenario

import (
	"context"
	"testing"

	Ω "github.com/onsi/gomega"
)

func Test_githubRepoHasReleaseWithTag(t *testing.T) {
	if isRunningInCI() {
		t.Skip("skip this step in CI. GitHub action credentials do not have access to crhntr/hello-release")
	}
	setup := func(t *testing.T) (context.Context, Ω.Gomega) {
		please := Ω.NewWithT(t)
		ctx := context.Background()
		err := checkoutMain(testTilePath)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		ctx = setTileRepoPath(ctx, testTilePath)
		ctx, err = loadGithubToken(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return ctx, please
	}

	t.Run("release exists", func(t *testing.T) {
		ctx, please := setup(t)
		err := githubRepoHasReleaseWithTag(ctx, "crhntr", "hello-release", "v0.1.5")
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("release does not exist", func(t *testing.T) {
		ctx, please := setup(t)
		err := githubRepoHasReleaseWithTag(ctx, "crhntr", "hello-release", "v99.99.99-banana")
		please.Expect(err).To(Ω.HaveOccurred())
	})
}
