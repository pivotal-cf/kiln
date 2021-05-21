package acceptance_test

import (
	"os/exec"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const GLOBAL_USAGE = `kiln
kiln helps you build ops manager compatible tiles

Usage: kiln [options] <command> [<args>]
  --help, -h     bool  prints this usage information (default: false)
  --version, -v  bool  prints the kiln release version (default: false)

Commands:
  bake                    bakes a tile
  compile-built-releases  compiles built releases and uploads them
  fetch                   fetches releases
  find-release-version    prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints
  help                    prints this usage information
  pre-process             preprocess yaml files
  publish                 publish tile on Pivnet
  sync-with-local         update the Kilnfile.lock based on local releases
  update-release          bumps a release to a new version
  update-stemcell         updates Kilnfile.lock with stemcell info
  upload-release          uploads a BOSH release to an s3 release_source
  version                 prints the kiln release version
`

const BAKE_USAGE = `kiln bake
Bakes tile metadata, stemcell, releases, and migrations into a format that can be consumed by OpsManager.

Usage: kiln [options] bake [<args>]
  --help, -h     bool  prints this usage information (default: false)
  --version, -v  bool  prints the kiln release version (default: false)

Command Arguments:
  --bosh-variables-directory, -vd    string (variadic)  path to a directory containing BOSH variables (default: bosh_variables)
  --embed, -e                        string (variadic)  path to files to include in the tile /embed directory
  --forms-directory, -f              string (variadic)  path to a directory containing forms (default: forms)
  --icon, -i                         string             path to icon file (default: icon.png)
  --instance-groups-directory, -ig   string (variadic)  path to a directory containing instance groups (default: instance_groups)
  --jobs-directory, -j               string (variadic)  path to a directory containing jobs (default: jobs)
  --kilnfile, -kf                    string             path to Kilnfile  (NOTE: mutually exclusive with --stemcell-directory) (default: Kilnfile)
  --metadata, -m                     string             path to the metadata file (default: base.yml)
  --metadata-only, -mo               bool               don't build a tile, output the metadata to stdout
  --migrations-directory, -md        string (variadic)  path to a directory containing migrations (default: migrations)
  --output-file, -o                  string             path to where the tile will be output
  --properties-directory, -pd        string (variadic)  path to a directory containing property blueprints (default: properties)
  --releases-directory, -rd          string (variadic)  path to a directory containing release tarballs (default: releases)
  --runtime-configs-directory, -rcd  string (variadic)  path to a directory containing runtime configs (default: runtime_configs)
  --sha256                           bool               calculates a SHA256 checksum of the output file
  --stemcell-tarball, -st            string             deprecated -- path to a stemcell tarball  (NOTE: mutually exclusive with --kilnfile)
  --stemcells-directory, -sd         string (variadic)  path to a directory containing stemcells  (NOTE: mutually exclusive with --kilnfile or --stemcell-tarball)
  --stub-releases, -sr               bool               skips importing release tarballs into the tile
  --variable, -vr                    string (variadic)  key value pairs of variables to interpolate
  --variables-file, -vf              string (variadic)  path to a file containing variables to interpolate
  --version, -v                      string             version of the tile
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
