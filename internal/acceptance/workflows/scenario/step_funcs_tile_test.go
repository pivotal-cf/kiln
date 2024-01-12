package scenario

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_theLockSpecifiesVersionForRelease(t *testing.T) {
	setup := func(t *testing.T) (context.Context, Gomega) {
		please := NewWithT(t)
		ctx := context.Background()
		err := checkoutMain(testTilePath)
		please.Expect(err).NotTo(HaveOccurred())
		ctx = setTileRepoPath(ctx, testTilePath)
		return ctx, please
	}

	t.Run("it matches the release version", func(t *testing.T) {
		ctx, please := setup(t)
		err := theLockSpecifiesVersionForRelease(ctx, "0.1.5", "hello-release")
		please.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("it does not match the release version", func(t *testing.T) {
		ctx, please := setup(t)
		err := theLockSpecifiesVersionForRelease(ctx, "9000.0.0", "hello-release")
		please.Expect(err).To(HaveOccurred())
	})
}
