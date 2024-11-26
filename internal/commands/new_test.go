package commands

import (
	"github.com/pivotal-cf/kiln/pkg/bake"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("new", func() {
	var (
		directory string
		n         *New

		args []string
	)

	BeforeEach(func() {
		var err error
		directory, err = os.MkdirTemp("", "kiln-new-*")
		if err != nil {
			log.Fatal(err)
		}
		n = new(New)
	})

	When("the tile flag is not set", func() {
		It("returns an error", func() {
			_, err := n.Setup(nil)
			Expect(err).To(MatchError(ContainSubstring("--tile")))
		})
	})

	When("only required arguments are set", func() {
		BeforeEach(func() {
			args = []string{
				"--tile", filepath.FromSlash("testdata/tile-0.1.2.pivotal"),
			}
		})
		It("does not error out", func() {
			_, err := n.Setup(args)
			Expect(err).NotTo(HaveOccurred())
		})
		It("sets some release sources", func() {
			k, _ := n.Setup(args)
			Expect(k.ReleaseSources).NotTo(BeEmpty())
		})
		It("sets the current working directory", func() {
			_, _ = n.Setup(args)
			Expect(n.Options.Dir).To(Equal("."))
		})
		It("does not set the product slug", func() {
			_, _ = n.Setup(args)
			Expect(n.Options.Slug).To(BeEmpty())
		})
		It("sset the bosh.io release source kilnfile template", func() {
			_, _ = n.Setup(args)
			Expect(n.Options.KilnfileTemplate).To(Equal(cargo.BOSHReleaseTarballSourceTypeBOSHIO))
		})
	})

	When("generating source", func() {
		BeforeEach(func() {
			args = []string{
				"--tile", filepath.FromSlash("testdata/tile-0.1.3.pivotal"),
				"--directory", directory,
			}
		})
		It("generates tile source", func() {
			Expect(n.Execute(args)).To(Succeed())

			Expect(filepath.Join(directory, bake.DefaultFilepathKilnfile)).To(BeAnExistingFile())
			Expect(filepath.Join(directory, bake.DefaultFilepathKilnfileLock)).To(BeAnExistingFile())
			Expect(filepath.Join(directory, bake.DefaultFilepathBaseYML)).To(BeAnExistingFile())
			Expect(filepath.Join(directory, bake.DefaultFilepathIconImage)).To(BeAnExistingFile())
		})
	})
})
