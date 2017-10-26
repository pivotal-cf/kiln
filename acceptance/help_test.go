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
  --help, -h  bool  prints this usage information (default: false)

Commands:
  bake  bakes a tile
  help  prints this usage information
`

const BAKE_USAGE = `kiln bake
Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.

Usage: kiln [options] bake [<args>]
  --help, -h  bool  prints this usage information (default: false)

Command Arguments:
  --releases-directory, -rd    string (variadic)  path to a directory containing release tarballs
  --migrations-directory, -md  string (variadic)  path to a directory containing migrations
  --embed, -e                  string (variadic)  path to files to include in the tile /embed directory
  --stemcell-tarball, -st      string             path to a stemcell tarball
  --metadata, -m               string             path to the metadata file
  --version, -v                string             version of the tile
  --product-name, -pn          string             product name
  --output-file, -o            string             path to where the tile will be output
  --stub-releases, -sr         bool               skips importing release tarballs into the tile
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
