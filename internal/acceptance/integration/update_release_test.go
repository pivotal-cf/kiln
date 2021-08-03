package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Context("Updating a release to a specific version", func() {
	var (
		kilnfileContents,
		previousKilnfileLock,
		kilnfileLockPath,
		kilnfilePath,
		releasesPath,
		varsFilePath,
		tmpDir string

		varsFileContents = os.Getenv("KILN_ACCEPTANCE_VARS_FILE_CONTENTS")
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		kilnfileLockPath = filepath.Join(tmpDir, "Kilnfile.lock")
		kilnfilePath = filepath.Join(tmpDir, "Kilnfile")
		releasesPath = filepath.Join(tmpDir, "releases")

		Expect(
			os.Mkdir(releasesPath, 0700),
		).To(Succeed())
	})

	AfterEach(func() {
		Expect(
			os.RemoveAll(tmpDir),
		).To(Succeed())
	})

	JustBeforeEach(func() {
		Expect(
			ioutil.WriteFile(kilnfilePath, []byte(kilnfileContents), 0600),
		).To(Succeed())

		Expect(
			ioutil.WriteFile(kilnfileLockPath, []byte(previousKilnfileLock), 0600),
		).To(Succeed())
	})

	Context("for public releases", func() {
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
					Releases: []cargo.ReleaseLock{
						{
							Name:         "loggregator-agent",
							Version:      "5.1.0",
							SHA1:         "a86e10219b0ed9b7b82f0610b7cdc03c13765722",
							RemoteSource: "bosh.io",
							RemotePath:   "https://bosh.io/d/github.com/cloudfoundry/loggregator-agent-release?v=5.1.0",
						},
						{
							Name:         "capi",
							Version:      "1.88.0",
							SHA1:         "7a7ef183de3252724b6f8e6ca39ad7cf4995fe27",
							RemoteSource: "bosh.io",
							RemotePath:   "https://bosh.io/d/github.com/cloudfoundry/capi-release?v=1.88.0",
						},
					},
					Stemcell: cargo.Stemcell{
						OS:      "some-os",
						Version: "4.5.6",
					},
				}))
		})

		When("--allow-only-publishable-releases is passed and no publishable release is available", func() {
			It("fails", func() {
				command := exec.Command(pathToMain, "update-release",
					"--allow-only-publishable-releases",
					"--name", "capi",
					"--version", "1.88.0",
					"--kilnfile", kilnfilePath,
					"--releases-directory", releasesPath)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 60*time.Second).Should(gexec.Exit(1))
				Expect(session.Err).To(gbytes.Say(`couldn't find "capi" 1.88.0`))
			})
		})
	})

	Context("for private releases (on S3)", func() {
		BeforeEach(func() {
			if varsFileContents == "" {
				Fail("please provide the KILN_ACCEPTANCE_VARS_FILE_CONTENTS environment variable")
			}

			kilnfileContents = `---
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: $(variable "aws_access_key_id")
  secret_access_key: $(variable "aws_secret_access_key")
  path_template: 2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
`
			previousKilnfileLock = `---
releases:
- name: "loggregator-agent"
  version: "5.1.0"
  sha1: "a86e10219b0ed9b7b82f0610b7cdc03c13765722"
  remote_source: compiled-releases
  remote_path: "2.8/loggregator-agent/loggregator-agent-5.1.0-ubuntu-xenial-456.30.tgz"
- name: capi
  sha1: "03ac801323cd23205dde357cc7d2dc9e92bc0c93"
  version: "1.87.0"
  remote_source: compiled-releases
  remote_path: "2.8/capi/capi-1.86.0-ubuntu-xenial-456.30.tgz"
stemcell_criteria:
  os: ubuntu-xenial
  version: '456.30'
`

			varsFilePath = filepath.Join(tmpDir, "variables.yml")

			Expect(
				ioutil.WriteFile(varsFilePath, []byte(varsFileContents), 0600),
			).To(Succeed())
		})

		It("updates the Kilnfile.lock", func() {
			command := exec.Command(pathToMain, "update-release",
				"--name", "capi",
				"--version", "1.86.0",
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--variables-file", varsFilePath,
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 60*time.Second).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("Updated capi to 1.86.0"))

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
							Version:      "5.1.0",
							SHA1:         "a86e10219b0ed9b7b82f0610b7cdc03c13765722",
							RemoteSource: "compiled-releases",
							RemotePath:   "2.8/loggregator-agent/loggregator-agent-5.1.0-ubuntu-xenial-456.30.tgz",
						},
						{
							Name:         "capi",
							Version:      "1.86.0",
							SHA1:         "32f40c3006e3b0b401b855da99cbd701c3c5be33",
							RemoteSource: "compiled-releases",
							RemotePath:   "2.8/capi/capi-1.86.0-ubuntu-xenial-456.30.tgz",
						},
					},
					Stemcell: cargo.Stemcell{
						OS:      "ubuntu-xenial",
						Version: "456.30",
					},
				}))
		})
	})
})
