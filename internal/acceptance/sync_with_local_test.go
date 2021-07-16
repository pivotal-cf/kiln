package acceptance_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Context("Syncing the Kilnfile.lock to releases on disk", func() {
	var (
		previousKilnfileLock string
		kilnfileLockPath     string
		kilnfilePath         string
		releasesDirPath      string
		expectedReleaseSHA   string
	)

	const (
		_readOnly      = 0600
		_readExecWrite = 0700
	)

	BeforeEach(func() {
		previousKilnfileLock = `---
releases:
- name: "loggregator-agent"
  version: "5.1.0"
  sha1: "a86e10219b0ed9b7b82f0610b7cdc03c13765722"
  remote_source: "bosh.io"
  remote_path: "https://bosh.io/d/github.com/cloudfoundry/loggregator-agent-release?v=5.1.0"
- name: capi
  sha1: "03ac801323cd23205dde357cc7d2dc9e92bc0c93"
  version: "1.87.0"
  remote_source: "bosh.io"
  remote_path: "https://bosh.io/d/github.com/cloudfoundry/capi-release?v=1.87.0"
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`

		kilnfileContents := `---
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: $(variable "aws_access_key_id")
  secret_access_key: $(variable "aws_secret_access_key")
  path_template: 2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
- type: bosh.io
`

		tmpDir, err := ioutil.TempDir("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		kilnfilePath = filepath.Join(tmpDir, "Kilnfile")
		kilnfileLockPath = kilnfilePath + ".lock"
		releasesDirPath = filepath.Join(tmpDir, "upload-releases")
		loggregatorReleaseDirPath := filepath.Join(releasesDirPath, "loggregator-agent")

		Expect(os.MkdirAll(loggregatorReleaseDirPath, _readExecWrite)).To(Succeed())
		Expect(ioutil.WriteFile(kilnfilePath, []byte(kilnfileContents), _readOnly)).To(Succeed())
		Expect(ioutil.WriteFile(kilnfileLockPath, []byte(previousKilnfileLock), _readOnly)).To(Succeed())

		expectedReleaseSHA, err = test_helpers.WriteReleaseTarball(
			fmt.Sprintf("%s/loggregator-agent-5.3.6.tgz", loggregatorReleaseDirPath),
			"loggregator-agent",
			"5.3.6",
			osfs.New(""))
		Expect(err).NotTo(HaveOccurred())
	})

	It("updates the Kilnfile.lock", func() {
		command := exec.Command(pathToMain, "sync-with-local",
			"--assume-release-source", "compiled-releases",
			"--kilnfile", kilnfilePath,
			"--releases-directory", releasesDirPath,
			"--variable", "aws_access_key_id=do_not_care",
			"--variable", "aws_secret_access_key=do_not_care",
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 60*time.Second).Should(gexec.Exit(0))
		Expect(session.Out).To(gbytes.Say("Updated loggregator-agent to 5.3.6"))

		var kilnfileLock cargo.KilnfileLock

		file, err := os.Open(kilnfileLockPath)
		Expect(err).NotTo(HaveOccurred())

		err = yaml.NewDecoder(file).Decode(&kilnfileLock)
		Expect(err).NotTo(HaveOccurred())

		Expect(kilnfileLock).To(Equal(
			cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         "loggregator-agent",
						Version:      "5.3.6",
						SHA1:         expectedReleaseSHA,
						RemoteSource: "compiled-releases",
						RemotePath:   "2.8/loggregator-agent/loggregator-agent-5.3.6-some-os-4.5.6.tgz",
					},
					{
						Name:         "capi",
						Version:      "1.87.0",
						SHA1:         "03ac801323cd23205dde357cc7d2dc9e92bc0c93",
						RemoteSource: "bosh.io",
						RemotePath:   "https://bosh.io/d/github.com/cloudfoundry/capi-release?v=1.87.0",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "some-os",
					Version: "4.5.6",
				},
			}))
	})
})
