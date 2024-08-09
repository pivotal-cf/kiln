package commands_test

import (
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"

	"github.com/pivotal-cf/kiln/internal/commands"
)

var _ = Describe("validate", func() {
	var (
		validate  commands.Validate
		directory billy.Filesystem
	)

	BeforeEach(func() {
		directory = memfs.New()
	})

	JustBeforeEach(func() {
		validate = commands.NewValidate(directory)
	})

	When("the kilnfile has two release_sources", func() {
		BeforeEach(func() {
			f, err := directory.Create("Kilnfile")
			Expect(err).NotTo(HaveOccurred())
			// language=yaml
			_, _ = io.WriteString(f, `---
release_sources:
  - type: "bosh.io"
  - type: "github"
`)
			_ = f.Close()
		})

		BeforeEach(func() {
			f, err := directory.Create("Kilnfile.lock")
			Expect(err).NotTo(HaveOccurred())
			_ = f.Close()
		})

		When("both types are in the allow list", func() {
			It("it does fail", func() {
				err := validate.Execute([]string{
					"--allow-release-source-type=bosh.io",
					"--allow-release-source-type=github",
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})
		When("both one of the types is not in the allow list", func() {
			It("it does fail", func() {
				err := validate.Execute([]string{
					"--allow-release-source-type=bosh.io",
				})
				Expect(err).To(MatchError(ContainSubstring("release source type not allowed: github")))
			})
		})
		When("the allow list is empty", func() {
			It("it does not fail", func() {
				err := validate.Execute([]string{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
