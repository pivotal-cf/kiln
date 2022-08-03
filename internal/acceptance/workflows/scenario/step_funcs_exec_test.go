package scenario

import (
	"context"
	Ω "github.com/onsi/gomega"
	"os/exec"
	"testing"
)

func Test_outputContainsSubstring(t *testing.T) {
	t.Run("stdout contains the string", func(t *testing.T) {
		please := Ω.NewWithT(t)
		ctx := context.Background()
		ctx = configureStandardFileDescriptors(ctx)
		_, err := runAndLogOnError(ctx, exec.Command("echo", "Hello, world!"), true)
		please.Expect(err).NotTo(Ω.HaveOccurred())

		err = outputContainsSubstring(ctx, "stdout", "world")
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("stderr contains the string", func(t *testing.T) {
		please := Ω.NewWithT(t)
		ctx := context.Background()
		ctx = configureStandardFileDescriptors(ctx)
		_, err := runAndLogOnError(ctx, exec.Command("bash", "-c", `echo "Hello, world!" > /dev/stderr`), true)
		please.Expect(err).NotTo(Ω.HaveOccurred())

		err = outputContainsSubstring(ctx, "stderr", "world")
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("stdout does not contain the substring", func(t *testing.T) {
		please := Ω.NewWithT(t)
		ctx := context.Background()
		ctx = configureStandardFileDescriptors(ctx)
		_, err := runAndLogOnError(ctx, exec.Command("echo", "Hello, world!"), true)
		please.Expect(err).NotTo(Ω.HaveOccurred())

		err = outputContainsSubstring(ctx, "stdout", "banana")
		please.Expect(err).To(Ω.MatchError(Ω.Equal("expected substring \"banana\" not found in: \"Hello, world!\"")))
	})
}
