package cargo_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func writeFile(fs billy.Filesystem, path string, contents string) error {
	file, err := fs.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

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
		kilnfileLoader   cargo.KilnfileLoader
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
    path_template: $( variable "path_template" )
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
path_template: "not-used"
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
			Expect(kilnfile).To(Equal(cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{
						Type:            "s3",
						Bucket:          "my-bucket",
						Region:          "middle-earth",
						AccessKeyId:     "id",
						SecretAccessKey: "key",
						PathTemplate:    "not-used",
					},
				},
			}))
		})

		It("correctly loads the Kilnfile.lock", func() {
			_, kilnfileLock, err := kilnfileLoader.LoadKilnfiles(filesystem, kilnfilePath, []string{variableFilePath}, variableStrings)
			Expect(err).NotTo(HaveOccurred())
			Expect(kilnfileLock).To(Equal(cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{{Name: "some-release", Version: "1.2.3"}},
				Stemcell: cargo.Stemcell{OS: "some-os", Version: "4.5.6"},
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

var _ = Describe("SaveKilnfileLock", func() {
	var (
		filesystem       billy.Filesystem
		kilnfilePath     string
		kilnfileLockPath string
		kilnfileLoader   cargo.KilnfileLoader
	)

	BeforeEach(func() {
		filesystem = memfs.New()

		kilnfilePath = "my-kilnfile"
		kilnfileLockPath = kilnfilePath + ".lock"
	})

	const validKilnfileLockContents = `
---
releases:
- name: release-A
  version: "1.2.3"
  remote_source: old-source
  remote_path: old-remote-path
  sha1: old-sha-1
stemcell_criteria:
  os: some-os
  version: "4.5.6"
`

	Context("happy path", func() {
		var updatedKilnfileLock cargo.KilnfileLock

		BeforeEach(func() {
			Expect(
				writeFile(filesystem, kilnfileLockPath, validKilnfileLockContents),
			).To(Succeed())

			updatedKilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         "release-A",
						Version:      "1.2.4",
						RemoteSource: "new-source",
						RemotePath:   "new-remote-path",
						SHA1:         "new-sha1",
					},
					{
						Name:         "release-B",
						Version:      "42",
						RemoteSource: "new-source2",
						RemotePath:   "new-remote-path2",
						SHA1:         "new-sha1-2",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "new-os",
					Version: "95",
				},
			}
		})

		It("correctly updates the Kilnfile.lock", func() {
			Expect(
				kilnfileLoader.SaveKilnfileLock(filesystem, kilnfilePath, updatedKilnfileLock),
			).To(Succeed())

			file, err := filesystem.Open(kilnfileLockPath)
			Expect(err).NotTo(HaveOccurred())

			var lockfileOnDisk cargo.KilnfileLock
			Expect(
				yaml.NewDecoder(file).Decode(&lockfileOnDisk),
			).To(Succeed())

			Expect(lockfileOnDisk).To(Equal(updatedKilnfileLock))
		})
	})

	When("reopening the Kilnfile.lock fails", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("very very bad")

			ogFilesystem := filesystem
			filesystem = fakeFilesystem{
				Filesystem: ogFilesystem,
				CreateFunc: func(path string) (billy.File, error) {
					return nil, expectedError
				},
			}
		})

		It("errors", func() {
			err := kilnfileLoader.SaveKilnfileLock(filesystem, "Kilnfile", cargo.KilnfileLock{})
			Expect(err).To(MatchError(ContainSubstring(expectedError.Error())))
		})
	})

	When("writing to the Kilnfile.lock fails", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("i don't feel so good")

			badFile := unwritableFile{err: expectedError}
			ogFilesystem := filesystem
			filesystem = fakeFilesystem{
				Filesystem: ogFilesystem,
				CreateFunc: func(path string) (billy.File, error) { return badFile, nil },
			}
		})

		It("errors", func() {
			err := kilnfileLoader.SaveKilnfileLock(filesystem, "Kilnfile", cargo.KilnfileLock{})
			Expect(err).To(MatchError(ContainSubstring(expectedError.Error())))
		})
	})
})

type fakeFilesystem struct {
	billy.Filesystem
	CreateFunc func(string) (billy.File, error)
}

func (fs fakeFilesystem) Create(path string) (billy.File, error) {
	return fs.CreateFunc(path)
}

type unwritableFile struct {
	billy.File
	err error
}

func (f unwritableFile) Write(_ []byte) (int, error) {
	return 0, f.err
}
