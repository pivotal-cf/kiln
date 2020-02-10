package integration_test

import (
	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Context("Updating the stemcell", func() {
	var (
		kilnfileLockPath,
		kilnfilePath,
		stemcellPath,
		releasesPath,
		varsFilePath,
		tmpDir string

		varsFileContents = os.Getenv("KILN_ACCEPTANCE_VARS_FILE_CONTENTS")
	)

	const (
		kilnfileContents = `---
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: $(variable "aws_access_key_id")
  secret_access_key: $(variable "aws_secret_access_key")
  path_template: 2.10/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
- type: bosh.io
`

		previousKilnfileLock = `---
releases:
- name: push-apps-manager-release
  version: 672.0.0
  remote_path: 2.10/push-apps-manager/push-apps-manager-release-672.0.0-ubuntu-xenial-456.30.tgz
  remote_source: compiled-releases
  sha1: old-sha1
- name: capi
  version: 1.89.0
  remote_path: 2.10/capi/capi-1.90.0-ubuntu-xenial-456.30.tgz
  remote_source: compiled-releases
  sha1: old-sha1
- name: cf-cli
  version: 1.23.0
  remote_path: https://bosh.io/d/github.com/bosh-packages/cf-cli-release?v=1.23.0
  remote_source: bosh.io
  sha1: cda431fa1e550c28bf6f5c82b3a3cf2c168771f2
stemcell_criteria:
  os: ubuntu-xenial
  version: '456.30'
`
	)

	BeforeEach(func() {
		if varsFileContents == "" {
			Fail("please provide the KILN_ACCEPTANCE_VARS_FILE_CONTENTS environment variable")
		}

		var err error
		tmpDir, err = ioutil.TempDir("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		kilnfileLockPath = filepath.Join(tmpDir, "Kilnfile.lock")
		kilnfilePath = filepath.Join(tmpDir, "Kilnfile")
		releasesPath = filepath.Join(tmpDir, "releases")
		stemcellPath = filepath.Join(tmpDir, "updated-stemcell.tgz")
		varsFilePath = filepath.Join(tmpDir, "variables.yml")

		_, err = test_helpers.WriteStemcellTarball(stemcellPath, "ubuntu-xenial", "621.51", osfs.New(""))
		Expect(err).NotTo(HaveOccurred())

		Expect(
			ioutil.WriteFile(kilnfilePath, []byte(kilnfileContents), 0600),
		).To(Succeed())

		Expect(
			ioutil.WriteFile(kilnfileLockPath, []byte(previousKilnfileLock), 0600),
		).To(Succeed())

		Expect(
			ioutil.WriteFile(varsFilePath, []byte(varsFileContents), 0600),
		).To(Succeed())

		Expect(
			os.Mkdir(releasesPath, 0700),
		).To(Succeed())
	})

	AfterEach(func() {
		Expect(
			os.RemoveAll(tmpDir),
		).To(Succeed())
	})

	It("updates the Kilnfile.lock", func() {
		command := exec.Command(pathToMain, "update-stemcell",
			"--stemcell-file", stemcellPath,
			"--kilnfile", kilnfilePath,
			"--releases-directory", releasesPath,
			"--variables-file", varsFilePath,
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 60*time.Second).Should(gexec.Exit(0))
		Expect(string(session.Out.Contents())).To(ContainSubstring(`Updating release "push-apps-manager-release" with stemcell ubuntu-xenial 621.51`))
		Expect(string(session.Out.Contents())).To(ContainSubstring(`Updating release "capi" with stemcell ubuntu-xenial 621.51`))
		Expect(string(session.Out.Contents())).To(ContainSubstring(`Updating release "cf-cli" with stemcell ubuntu-xenial 621.51`))
		Expect(string(session.Out.Contents())).To(ContainSubstring(`Finished updating Kilnfile.lock`))

		var kilnfileLock cargo.KilnfileLock

		file, err := os.Open(kilnfileLockPath)
		Expect(err).NotTo(HaveOccurred())

		err = yaml.NewDecoder(file).Decode(&kilnfileLock)
		Expect(err).NotTo(HaveOccurred())

		upgradeStemcell := cargo.ReleaseLock{
			Name:         "push-apps-manager-release",
			Version:      "672.0.0",
			SHA1:         "dd51c8b05cce325d68b7fc0c0c4e563b72c63006",
			RemoteSource: "compiled-releases",
			RemotePath:   "2.10/push-apps-manager/push-apps-manager-release-672.0.0-ubuntu-xenial-621.51.tgz",
		}
		notCompiledAgainstNewStemcell := cargo.ReleaseLock{
			Name:         "capi",
			Version:      "1.89.0",
			SHA1:         "472966da017bda3118eb9cf71f9bcca1c23c344a",
			RemoteSource: "bosh.io",
			RemotePath:   "https://bosh.io/d/github.com/cloudfoundry/capi-release?v=1.89.0",
		}
		remainsBuilt := cargo.ReleaseLock{
			Name:         "cf-cli",
			Version:      "1.23.0",
			SHA1:         "cda431fa1e550c28bf6f5c82b3a3cf2c168771f2",
			RemoteSource: "bosh.io",
			RemotePath:   "https://bosh.io/d/github.com/bosh-packages/cf-cli-release?v=1.23.0",
		}

		Expect(kilnfileLock).To(Equal(
			cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					upgradeStemcell, notCompiledAgainstNewStemcell, remainsBuilt,
				},
				Stemcell: cargo.Stemcell{
					OS:      "ubuntu-xenial",
					Version: "621.51",
				},
			}))
	})
})
