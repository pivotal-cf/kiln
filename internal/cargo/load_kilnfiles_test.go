package cargo_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/internal/cargo"
	"io/ioutil"
	"path/filepath"
)

var _ = Describe("LoadKilnfiles", func() {
	var (
		kilnfilePath     string
		kilnfileLockPath string
		variableFilePath string
		variableStrings  []string
	)

	BeforeEach(func() {
		tmpDir, err := ioutil.TempDir("", "load-kilnfiles-test")
		Expect(err).NotTo(HaveOccurred())

		kilnfilePath = filepath.Join(tmpDir, "my-kilnfile")
		kilnfileLockPath = kilnfilePath + ".lock"
		variableFilePath = filepath.Join(tmpDir, "variable-file.yml")

		variableStrings = []string{
			"access_key=id",
			"secret_key=key",
		}
	})

	const validKilnfileContents = `
---
release_sources:
  - type: s3
    compiled: true
    bucket: $( variable "bucket" )
    region: $( variable "region" )
    access_key_id: $( variable "access_key" )
    secret_access_key: $( variable "secret_key" )
    regex: $( variable "regex" )
`

	const validKilnfileLockContents = `
---
releases:
- name: some-release
  version: "1.2.3"
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`

	const validVariableFileContents = `
---
bucket: my-bucket
region: middle-earth
regex: "^.*$"
`

	Context("happy path", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte(validKilnfileContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(kilnfileLockPath, []byte(validKilnfileLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte(validVariableFileContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("correctly loads the Kilnfile", func() {
			kilnfile, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).NotTo(HaveOccurred())
			Expect(kilnfile).To(Equal(Kilnfile{
				ReleaseSources: []ReleaseSourceConfig{
					{
						Type:            "s3",
						Compiled:        true,
						Bucket:          "my-bucket",
						Region:          "middle-earth",
						AccessKeyId:     "id",
						SecretAccessKey: "key",
						Regex:           "^.*$",
					},
				},
			}))
		})

		It("correctly loads the Kilnfile.lock", func() {
			_, kilnfileLock, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).NotTo(HaveOccurred())
			Expect(kilnfileLock).To(Equal(KilnfileLock{
				Releases: []Release{{Name: "some-release", Version: "1.2.3"}},
				Stemcell: Stemcell{OS: "some-os", Version: "4.5.6"},
			}))
		})
	})

	When("the Kilnfile doesn't exist", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfileLockPath, []byte(validKilnfileLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte(validVariableFileContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("no such file")))
		})
	})

	When("the Kilnfile.lock doesn't exist", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte(validKilnfileContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte(validVariableFileContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("no such file")))
		})
	})

	When("the variable file doesn't exist", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte(validKilnfileContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(kilnfileLockPath, []byte(validKilnfileLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("no such file")))
		})
	})

	When("the Kilnfile is invalid YAML", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte("some-random-string"), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(kilnfileLockPath, []byte(validKilnfileLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte(validVariableFileContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	When("the Kilnfile.lock is invalid YAML", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte(validKilnfileContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(kilnfileLockPath, []byte("some-random-string"), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte(validVariableFileContents), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	When("the variables file is invalid YAML", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte(validKilnfileContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(kilnfileLockPath, []byte(validKilnfileLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte("invalid-yaml"), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	When("interpolation fails", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(kilnfilePath, []byte(validKilnfileContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(kilnfileLockPath, []byte(validKilnfileLockContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(variableFilePath, []byte("{}"), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := LoadKilnfiles(kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("could not find variable")))
		})
	})
})
