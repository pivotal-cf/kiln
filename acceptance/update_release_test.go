package acceptance_test

import (
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Context("Updating a release to a specific version", func() {
	var kilnfileContents, previousKilnfileLock, kilnfileLockPath, kilnfilePath, releasesPath string
	BeforeEach(func() {
		kilnfileContents = `---
release_sources:
- type: bosh.io
`
		previousKilnfileLock = `---
releases:
- name: "loggregator-agent"
  version: "5.1.0"
  sha1: "a86e10219b0ed9b7b82f0610b7cdc03c13765722"
- name: capi
  sha1: "03ac801323cd23205dde357cc7d2dc9e92bc0c93"
  version: "1.87.0"
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`
		tmpDir, err := ioutil.TempDir("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		kilnfileLockPath = filepath.Join(tmpDir, "Kilnfile.lock")
		kilnfilePath = filepath.Join(tmpDir, "Kilnfile")
		releasesPath = filepath.Join(tmpDir, "releases")
		ioutil.WriteFile(kilnfilePath, []byte(kilnfileContents), 0600)
		ioutil.WriteFile(kilnfileLockPath, []byte(previousKilnfileLock), 0600)
		os.Mkdir(releasesPath, 0700)
	})

	It("updates the Kilnfile.lock", func() {
		command := exec.Command(pathToMain, "update-release",
			"--name", "capi",
			"--version", "1.88.0",
			"--kilnfile", kilnfilePath,
			"--releases-directory", releasesPath)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 60*time.Second).Should(gexec.Exit(0))
		Expect(session.Out).To(gbytes.Say("Updated capi to 1.88.0"))

		var kilnfileLock cargo.KilnfileLock

		file, err := os.Open(kilnfileLockPath)
		Expect(err).NotTo(HaveOccurred())

		err = yaml.NewDecoder(file).Decode(&kilnfileLock)
		Expect(err).NotTo(HaveOccurred())

		Expect(kilnfileLock).To(Equal(
			cargo.KilnfileLock{
				Releases: []cargo.Release{
					{Name: "loggregator-agent", Version: "5.1.0", SHA1: "a86e10219b0ed9b7b82f0610b7cdc03c13765722"},
					{Name: "capi", Version: "1.88.0", SHA1: "7a7ef183de3252724b6f8e6ca39ad7cf4995fe27"},
				},
				Stemcell: cargo.Stemcell{
					OS:      "some-os",
					Version: "4.5.6",
				},
			}))
	})
})
