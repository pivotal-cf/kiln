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
  bake  builds a tile
  help  prints this usage information
`

const BAKE_USAGE = `kiln bake
Builds a tile to be uploaded to OpsMan from provided inputs.

Usage: kiln [options] bake [<args>]
  -h, --help  bool  prints this usage information (default: false)

Command Arguments:
  -rd, --releases-directory       string  path to the release tarballs directory
  -m, --migrations-directory      slice   path to the migrations directory
  -cm, --content-migration        slice   location of the content migration file
  -bcm, --base-content-migration  string  location of the base content migration file
  -st, --stemcell-tarball         string  location of the stemcell tarball
  -h, --handcraft                 string  location of the handcraft file
  -v, --version                   string  version for the filename
  -fv, --final-version            string  final version of the tile
  -pn, --product-name             string  product name
  -fp, --filename-prefix          string  prefix used for filename
  -o, --output-dir                string  output directory
  -sr, --stub-releases            bool    don't include release tarballs
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
