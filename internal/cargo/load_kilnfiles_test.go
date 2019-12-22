package cargo_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

func writeFile(fs billy.Filesystem, path string, contents string) error {
	file, err := fs.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write([]byte(contents))
	return err
}

var _ = Describe("LoadKilnfiles", func() {
	var (
		filesystem       billy.Filesystem
		kilnfilePath     string
		kilnfileLockPath string
		variableFilePath string
		variableStrings  []string
		kilnfileLoader   KilnfileLoader
	)

	BeforeEach(func() {
		filesystem = memfs.New()

		kilnfilePath = "my-kilnfile"
		kilnfileLockPath = kilnfilePath + ".lock"
		variableFilePath = "variable-file.yml"

		variableStrings = []string{
			"access_key=id",
			"secret_key=key",
		}

		kilnfileLoader = KilnfileLoader{}
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
			err := writeFile(filesystem, kilnfilePath, validKilnfileContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, validVariableFileContents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("correctly loads the Kilnfile", func() {
			kilnfile, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
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
			_, kilnfileLock, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).NotTo(HaveOccurred())
			Expect(kilnfileLock).To(Equal(KilnfileLock{
				Releases: []ReleaseLock{{Name: "some-release", Version: "1.2.3"}},
				Stemcell: Stemcell{OS: "some-os", Version: "4.5.6"},
			}))
		})
	})

	When("the Kilnfile doesn't exist", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, validVariableFileContents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("file does not exist")))
		})
	})

	When("the Kilnfile.lock doesn't exist", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfilePath, validKilnfileContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, validVariableFileContents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("file does not exist")))
		})
	})

	When("the variable file doesn't exist", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfilePath, validKilnfileContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("file does not exist")))
		})
	})

	When("the Kilnfile is invalid YAML", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfilePath, "some-random-string")
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, validVariableFileContents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	When("the Kilnfile.lock is invalid YAML", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfilePath, validKilnfileContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, kilnfileLockPath, "some-random-string")
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, validVariableFileContents)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	When("the variables file is invalid YAML", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfilePath, validKilnfileContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, "invalid-yaml")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})

	When("interpolation fails", func() {
		BeforeEach(func() {
			err := writeFile(filesystem, kilnfilePath, validKilnfileContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents)
			Expect(err).NotTo(HaveOccurred())

			err = writeFile(filesystem, variableFilePath, "{}")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error", func() {
			_, _, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).To(MatchError(ContainSubstring("could not find variable")))
		})
	})
})
