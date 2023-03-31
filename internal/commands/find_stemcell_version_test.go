package commands_test

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/internal/pivnet"
)

var _ = Describe("Find the stemcell version", func() {
	var (
		findStemcellVersion commands.FindStemcellVersion
		logger              *log.Logger

		writer strings.Builder

		fetchExecuteArgs     []string
		executeErr           error
		someKilnfilePath     string
		someKilnfileLockPath string
		kilnfileContents     string
		lockContents         string
		pivnetService        *pivnet.Service
		serverMock           *fakes.RoundTripper
		simpleRequest        *http.Request
		requestErr           error
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(&writer, "", 0)

			pivnetService = new(pivnet.Service)
			simpleRequest, _ = http.NewRequest(http.MethodGet, "/", nil)

			serverMock = &fakes.RoundTripper{}
			serverMock.Results.Res = &http.Response{}
			pivnetService.Client = &http.Client{
				Transport: serverMock,
			}

			kilnfileContents = `
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: my-access-key-id
  secret_access_key: my-secret-access-key
  path_template: 2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
stemcell_criteria:
  os: ubuntu-xenial
  version: "~456"
`
			lockContents = `
---
releases:
- name: some-release
  version: "1.2.3"
  remote_source:
  remote_path: my-remote-path
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`
		})

		JustBeforeEach(func() {
			_, requestErr = pivnetService.Do(simpleRequest)
			Expect(requestErr).NotTo(HaveOccurred())

			tmpDir, err := os.MkdirTemp("", "fetch-stemcell-test")
			Expect(err).NotTo(HaveOccurred())

			someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")

			err = os.WriteFile(someKilnfilePath, []byte(kilnfileContents), 0o644)
			Expect(err).NotTo(HaveOccurred())

			someKilnfileLockPath = filepath.Join(tmpDir, "Kilnfile.lock")
			err = os.WriteFile(someKilnfileLockPath, []byte(lockContents), 0o644)
			Expect(err).NotTo(HaveOccurred())

			findStemcellVersion = commands.NewFindStemcellVersion(logger, pivnetService)

			fetchExecuteArgs = []string{
				"--kilnfile", someKilnfilePath,
			}
			executeErr = findStemcellVersion.Execute(fetchExecuteArgs)
		})

		When("stemcell criteria does not exist in the kilnfile", func() {
			BeforeEach(func() {
				kilnfileContents = `
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: my-access-key-id
  secret_access_key: my-secret-access-key
  path_template: 2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
`
			})
			It("returns the stemcell os info missing error message", func() {
				Expect(executeErr).To(HaveOccurred())
				Expect(executeErr).To(MatchError(ContainSubstring(commands.ErrStemcellOSInfoMustBeValid)))
			})
		})

		When("stemcell major version does not exist in the kilnfile", func() {
			BeforeEach(func() {
				kilnfileContents = `
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: my-access-key-id
  secret_access_key: my-secret-access-key
  path_template: 2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
stemcell_criteria:
  os: ubuntu-xenial
`
			})

			It("returns stemcell major version missing error message", func() {
				Expect(executeErr).To(HaveOccurred())
				Expect(executeErr).To(MatchError(ContainSubstring(commands.ErrStemcellMajorVersionMustBeValid)))
			})
		})

		When("stemcell criteria has a constraint", func() {
			BeforeEach(func() {
				kilnfileContents = `
release_sources:
- type: s3
  bucket: compiled-releases
  region: us-west-1
  access_key_id: my-access-key-id
  secret_access_key: my-secret-access-key
  path_template: 2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
  publishable: true
stemcell_criteria:
  os: ubuntu-xenial
  version: "<456.100"
`
				serverMock.Results.Res.Body = fakes.NewReadCloser(`{"releases":[{"version": "456.98"},{"version": "456.99"},{"version": "456.100"}]}`)
				serverMock.Results.Res.StatusCode = http.StatusOK
				serverMock.Results.Err = nil
			})
			It("returns the latest stemcell version", func() {
				Expect(executeErr).NotTo(HaveOccurred())
				Expect((&writer).String()).To(ContainSubstring("\"456.99\""))
				Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"network.pivotal.io\""))
				Expect((&writer).String()).To(ContainSubstring("\"source\":\"Tanzunet\""))
			})
		})

		When("stemcell OS and major version is specified", func() {
			When("a new stemcell exists", func() {
				BeforeEach(func() {
					serverMock.Results.Res.Body = fakes.NewReadCloser(`{"releases":[{"version": "456.118"}]}`)
					serverMock.Results.Res.StatusCode = http.StatusOK
					serverMock.Results.Err = nil
				})

				It("returns the latest stemcell version", func() {
					Expect(executeErr).NotTo(HaveOccurred())
					Expect((&writer).String()).To(ContainSubstring("\"456.118\""))
					Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"network.pivotal.io\""))
					Expect((&writer).String()).To(ContainSubstring("\"source\":\"Tanzunet\""))
				})
			})
		})
	})
})
