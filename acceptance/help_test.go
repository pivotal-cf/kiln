package acceptance

import (
	"os/exec"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const GLOBAL_USAGE = `kiln
kiln helps you build ops manager compatible tiles

Usage: kiln [options] <command> [<args>]
  -h, --help  bool  prints this usage information (default: false)

Commands:
  bake  bakes a tile
  help  prints this usage information
`

const BAKE_USAGE = `kiln bake
Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.

Usage: kiln [options] bake [<args>]
  -h, --help  bool  prints this usage information (default: false)

Command Arguments:
  -rd, --releases-directory    slice   path to a directory containing release tarballs
  -md, --migrations-directory  slice   path to a directory containing migrations
  -e, --embed                  slice   path to files to include in the tile /embed directory
  -st, --stemcell-tarball      string  path to a stemcell tarball
  -m, --metadata               string  path to the metadata file
  -v, --version                string  version of the tile
  -pn, --product-name          string  product name
  -o, --output-file            string  path to where the tile will be output
  -sr, --stub-releases         bool    skips importing release tarballs into the tile
`

var _ = Describe("help", func() {
	Context("when given no command at all", func() {
		It("prints the global usage", func() {
			command := exec.Command(pathToMain)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(GLOBAL_USAGE))
		})
	})

	Context("when given the -h short flag", func() {
		It("prints the global usage", func() {
			command := exec.Command(pathToMain, "-h")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(GLOBAL_USAGE))
		})
	})

	Context("when given the --help long flag", func() {
		It("prints the global usage", func() {
			command := exec.Command(pathToMain, "--help")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(GLOBAL_USAGE))
		})
	})

	Context("when given the help command", func() {
		It("prints the global usage", func() {
			command := exec.Command(pathToMain, "help")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(GLOBAL_USAGE))
		})
	})

	Context("when given a command", func() {
		It("prints the usage for that command", func() {
			command := exec.Command(pathToMain, "help", "bake")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(BAKE_USAGE))
		})
	})
})
