package scenario

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_theLockSpecifiesVersionForRelease(t *testing.T) {
	setup := func(t *testing.T) (context.Context, Gomega) {
		ctx := context.Background()

		dir, err := copyTileDirectory(t.TempDir(), filepath.Join("..", "testdata", "tiles", "v2"))
		if err != nil {
			t.Fatal(err)
		}

		ctx = setTileRepoPath(ctx, dir)
		return ctx, NewWithT(t)
	}

	t.Run("it matches the release version", func(t *testing.T) {
		ctx, please := setup(t)
		err := theLockSpecifiesVersionForRelease(ctx, "0.2.3", "hello-release")
		please.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("it does not match the release version", func(t *testing.T) {
		ctx, please := setup(t)
		err := theLockSpecifiesVersionForRelease(ctx, "9000.0.0", "hello-release")
		please.Expect(err).To(HaveOccurred())
	})
}
