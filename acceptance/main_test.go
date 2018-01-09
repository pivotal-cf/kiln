package acceptance

import (
	"archive/zip"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("kiln", func() {
	var (
		metadata                         string
		otherReleasesDirectory           string
		outputFile                       string
		someIconPath                     string
		somePropertiesDirectory          string
		someReleasesDirectory            string
		someRuntimeConfigsDirectory      string
		someVariablesDirectory           string
		someFormsDirectory               string
		someOtherFormsDirectory          string
		someInstanceGroupsDirectory      string
		someOtherInstanceGroupsDirectory string
		someJobsDirectory                string
		someOtherJobsDirectory           string
		stemcellTarball                  string
		tmpDir                           string
		diegoSHA1                        string
		cfSHA1                           string
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		tileDir, err := ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		outputFile = filepath.Join(tileDir, "cool-product-1.2.3-build.4.pivotal")

		someIconFile, err := ioutil.TempFile("", "icon")
		Expect(err).NotTo(HaveOccurred())
		defer someIconFile.Close()
		someIconPath = someIconFile.Name()

		someImageData := "i-am-some-image"
		_, err = someIconFile.Write([]byte(someImageData))
		Expect(err).NotTo(HaveOccurred())

		somePropertiesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someRuntimeConfigsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someVariablesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someFormsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someOtherFormsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someInstanceGroupsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someOtherInstanceGroupsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someJobsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someOtherJobsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		cfReleaseManifest := `---
name: cf
version: 235
`
		err = ioutil.WriteFile(filepath.Join(somePropertiesDirectory, "some-templated-property.yml"), []byte(`---
name: some_templated_property_blueprint
type: boolean
configurable: false
default: true
`), 0644)

		Expect(err).NotTo(HaveOccurred())

		_, err = ioutil.TempFile(someReleasesDirectory, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = createTarball(someReleasesDirectory, "cf-release-235.0.0-3215.4.0.tgz", "release.MF", cfReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		f, err := os.Open(filepath.Join(someReleasesDirectory, "cf-release-235.0.0-3215.4.0.tgz"))
		Expect(err).NotTo(HaveOccurred())

		hash := sha1.New()
		_, err = io.Copy(hash, f)
		Expect(err).NotTo(HaveOccurred())

		cfSHA1 = fmt.Sprintf("%x", hash.Sum(nil))

		diegoReleaseManifest := `---
name: diego
version: 0.1467.1
key: value
`

		_, err = createTarball(otherReleasesDirectory, "diego-release-0.1467.1-3215.4.0.tgz", "release.MF", diegoReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		f, err = os.Open(filepath.Join(otherReleasesDirectory, "diego-release-0.1467.1-3215.4.0.tgz"))
		Expect(err).NotTo(HaveOccurred())

		hash = sha1.New()
		_, err = io.Copy(hash, f)
		Expect(err).NotTo(HaveOccurred())

		diegoSHA1 = fmt.Sprintf("%x", hash.Sum(nil))

		notATarball := filepath.Join(someReleasesDirectory, "not-a-tarball.txt")
		_ = ioutil.WriteFile(notATarball, []byte(`this is not a tarball`), 0644)
		stemcellManifest := `---
version: "3215.4"
operating_system: ubuntu-trusty
`

		stemcellTarball, err = createTarball(tmpDir, "stemcell.tgz", "stemcell.MF", stemcellManifest)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someFormsDirectory, "some-config.yml"), []byte(`---
name: some-config
label: some-form-label
description: some-form-description
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someFormsDirectory, "some-other-config.yml"), []byte(`---
name: some-other-config
label: some-other-form-label
description: some-other-form-description
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someOtherFormsDirectory, "some-more-config.yml"), []byte(`---
name: some-more-config
label: some-form-label
description: some-form-description
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someInstanceGroupsDirectory, "some-instance-group.yml"), []byte(`---
name: some-instance-group
label: Some Instance Group
templates:
- $( job "some-job" )
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someOtherInstanceGroupsDirectory, "some-other-instance-group.yml"), []byte(`---
name: some-other-instance-group
label: Some Other Instance Group
templates:
- $( job "some-job-alias" )
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someJobsDirectory, "some-job.yml"), []byte(`---
name: some-job
release: some-release
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someOtherJobsDirectory, "some-other-job.yml"), []byte(`---
name: some-other-job
alias: some-job-alias
release: some-other-release
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someRuntimeConfigsDirectory, "some-runtime-config.yml"), []byte(`---
name: some-runtime-config
runtime_config: |
  releases:
  - name: some-addon
    version: some-addon-version
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someVariablesDirectory, "variable-1.yml"), []byte(`---
variables:
- name: variable-1
  type: certificate
  options:
    some_option: Option value
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someVariablesDirectory, "variable-2.yml"), []byte(`---
variables:
- name: variable-2
  type: password
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		metadata = filepath.Join(tmpDir, "metadata.yml")
		err = ioutil.WriteFile(metadata, untemplatedMetadata, 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("generates a tile with the correct metadata", func() {
		command := exec.Command(pathToMain,
			"bake",
			"--forms-directory", someFormsDirectory,
			"--forms-directory", someOtherFormsDirectory,
			"--icon", someIconPath,
			"--instance-groups-directory", someInstanceGroupsDirectory,
			"--instance-groups-directory", someOtherInstanceGroupsDirectory,
			"--jobs-directory", someJobsDirectory,
			"--jobs-directory", someOtherJobsDirectory,
			"--metadata", metadata,
			"--migrations-directory", "fixtures/extra-migrations",
			"--migrations-directory", "fixtures/migrations",
			"--output-file", outputFile,
			"--properties-directory", somePropertiesDirectory,
			"--releases-directory", otherReleasesDirectory,
			"--releases-directory", someReleasesDirectory,
			"--runtime-configs-directory", someRuntimeConfigsDirectory,
			"--stemcell-tarball", stemcellTarball,
			"--bosh-variables-directory", someVariablesDirectory,
			"--variable", "some-variable=some-variable-value",
			"--version", "1.2.3",
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		archive, err := os.Open(outputFile)
		Expect(err).NotTo(HaveOccurred())

		archiveInfo, err := archive.Stat()
		Expect(err).NotTo(HaveOccurred())

		zr, err := zip.NewReader(archive, archiveInfo.Size())
		Expect(err).NotTo(HaveOccurred())

		var file io.ReadCloser
		for _, f := range zr.File {
			if f.Name == "metadata/cool-product-name.yml" {
				file, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
				break
			}
		}

		Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
		metadataContents, err := ioutil.ReadAll(file)
		Expect(err).NotTo(HaveOccurred())

		renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
		Expect(metadataContents).To(MatchYAML(renderedYAML))

		var (
			archivedMigration1 io.ReadCloser
			archivedMigration2 io.ReadCloser
			archivedMigration3 io.ReadCloser
		)

		for _, f := range zr.File {
			if f.Name == "migrations/v1/201603041539_custom_buildpacks.js" {
				archivedMigration1, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
			}

			if f.Name == "migrations/v1/201603071158_auth_enterprise_sso.js" {
				archivedMigration2, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
			}

			if f.Name == "migrations/v1/some_migration.js" {
				archivedMigration3, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
			}
		}

		contents, err := ioutil.ReadAll(archivedMigration1)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("custom-buildpack-migration\n"))

		contents, err = ioutil.ReadAll(archivedMigration2)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("auth-enterprise-sso-migration\n"))

		contents, err = ioutil.ReadAll(archivedMigration3)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("some_migration\n"))

		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Creating metadata for %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say("Reading release manifests"))
		Eventually(session.Out).Should(gbytes.Say("Reading stemcell manifest"))
		Eventually(session.Out).Should(gbytes.Say("Marshaling metadata file..."))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Building %s", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding metadata/cool-product-name.yml to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603041539_custom_buildpacks.js to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603071158_auth_enterprise_sso.js to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Out).ShouldNot(gbytes.Say(fmt.Sprintf("Adding releases/not-a-tarball.txt to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Calculating md5 sum of %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say("Calculated md5 sum: [0-9a-f]{32}"))
	})

	Context("when the --stub-releases flag is specified", func() {
		It("creates a tile with empty release tarballs", func() {
			command := exec.Command(pathToMain,
				"bake",
				"--forms-directory", someFormsDirectory,
				"--forms-directory", someOtherFormsDirectory,
				"--icon", someIconPath,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", someReleasesDirectory,
				"--releases-directory", otherReleasesDirectory,
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--instance-groups-directory", someOtherInstanceGroupsDirectory,
				"--jobs-directory", someJobsDirectory,
				"--jobs-directory", someOtherJobsDirectory,
				"--properties-directory", somePropertiesDirectory,
				"--runtime-configs-directory", someRuntimeConfigsDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--stub-releases",
				"--variable", "some-variable=some-variable-value",
				"--version", "1.2.3",
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			archive, err := os.Open(outputFile)
			Expect(err).NotTo(HaveOccurred())

			archiveInfo, err := archive.Stat()
			Expect(err).NotTo(HaveOccurred())

			zr, err := zip.NewReader(archive, archiveInfo.Size())
			Expect(err).NotTo(HaveOccurred())

			for _, f := range zr.File {
				if f.Name == "releases/cf-release-235.0.0-3215.4.0.tgz" {
					Expect(f.UncompressedSize64).To(Equal(uint64(0)))
				}

				if f.Name == "releases/diego-release-0.1467.1-3215.4.0.tgz" {
					Expect(f.UncompressedSize64).To(Equal(uint64(0)))
				}
			}
		})
	})

	Context("when no migrations are provided", func() {
		It("creates empty migrations folder", func() {
			command := exec.Command(pathToMain,
				"bake",
				"--forms-directory", someFormsDirectory,
				"--forms-directory", someOtherFormsDirectory,
				"--icon", someIconPath,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", someReleasesDirectory,
				"--releases-directory", otherReleasesDirectory,
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--instance-groups-directory", someOtherInstanceGroupsDirectory,
				"--jobs-directory", someJobsDirectory,
				"--jobs-directory", someOtherJobsDirectory,
				"--properties-directory", somePropertiesDirectory,
				"--runtime-configs-directory", someRuntimeConfigsDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--variable", "some-variable=some-variable-value",
				"--version", "1.2.3",
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			archive, err := os.Open(outputFile)
			Expect(err).NotTo(HaveOccurred())

			archiveInfo, err := archive.Stat()
			Expect(err).NotTo(HaveOccurred())

			zr, err := zip.NewReader(archive, archiveInfo.Size())
			Expect(err).NotTo(HaveOccurred())

			var emptyMigrationsFolderMode os.FileMode
			for _, f := range zr.File {
				if f.Name == "migrations/v1/" {
					emptyMigrationsFolderMode = f.Mode()
					break
				}
			}

			Expect(emptyMigrationsFolderMode.IsDir()).To(BeTrue())

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Creating empty migrations folder in %s...", outputFile)))
		})
	})

	Context("when the --embed flag is specified", func() {
		Context("when only file paths are specified", func() {
			It("creates a tile with the specified file copied into the embed directory", func() {
				someFileToEmbed := filepath.Join(tmpDir, "some-file-to-embed")
				otherFileToEmbed := filepath.Join(tmpDir, "other-file-to-embed")

				err := ioutil.WriteFile(someFileToEmbed, []byte("content-of-some-file"), 0600)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(otherFileToEmbed, []byte("content-of-other-file"), 0755)
				Expect(err).NotTo(HaveOccurred())

				command := exec.Command(pathToMain,
					"bake",
					"--forms-directory", someFormsDirectory,
					"--forms-directory", someOtherFormsDirectory,
					"--embed", otherFileToEmbed,
					"--embed", someFileToEmbed,
					"--icon", someIconPath,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--releases-directory", someReleasesDirectory,
					"--releases-directory", otherReleasesDirectory,
					"--instance-groups-directory", someInstanceGroupsDirectory,
					"--instance-groups-directory", someOtherInstanceGroupsDirectory,
					"--jobs-directory", someJobsDirectory,
					"--jobs-directory", someOtherJobsDirectory,
					"--properties-directory", somePropertiesDirectory,
					"--runtime-configs-directory", someRuntimeConfigsDirectory,
					"--stemcell-tarball", stemcellTarball,
					"--stub-releases",
					"--variable", "some-variable=some-variable-value",
					"--version", "1.2.3",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0))

				archive, err := os.Open(outputFile)
				Expect(err).NotTo(HaveOccurred())

				archiveInfo, err := archive.Stat()
				Expect(err).NotTo(HaveOccurred())

				zr, err := zip.NewReader(archive, archiveInfo.Size())
				Expect(err).NotTo(HaveOccurred())

				seenSomeFile := false
				seenOtherFile := false
				for _, f := range zr.File {
					if f.Name == "embed/some-file-to-embed" {
						seenSomeFile = true
						r, err := f.Open()
						Expect(err).NotTo(HaveOccurred())

						content, err := ioutil.ReadAll(r)
						Expect(err).NotTo(HaveOccurred())

						Expect(content).To(Equal([]byte("content-of-some-file")))
					}

					if f.Name == "embed/other-file-to-embed" {
						seenOtherFile = true
						r, err := f.Open()
						Expect(err).NotTo(HaveOccurred())

						content, err := ioutil.ReadAll(r)
						Expect(err).NotTo(HaveOccurred())

						mode := f.FileHeader.Mode()
						Expect(mode).To(Equal(os.FileMode(0755)))

						Expect(content).To(Equal([]byte("content-of-other-file")))
					}
				}

				Expect(seenSomeFile).To(BeTrue())
				Expect(seenOtherFile).To(BeTrue())
			})
		})

		Context("when a directory is specified", func() {
			It("embeds the root directory and retains its structure", func() {
				dirToAdd := filepath.Join(tmpDir, "some-dir")
				nestedDir := filepath.Join(dirToAdd, "some-nested-dir")
				someFileToEmbed := filepath.Join(nestedDir, "some-file-to-embed")

				err := os.MkdirAll(nestedDir, 0700)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(someFileToEmbed, []byte("content-of-some-file"), 0600)
				Expect(err).NotTo(HaveOccurred())

				command := exec.Command(pathToMain,
					"bake",
					"--forms-directory", someFormsDirectory,
					"--forms-directory", someOtherFormsDirectory,
					"--embed", dirToAdd,
					"--icon", someIconPath,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--releases-directory", someReleasesDirectory,
					"--releases-directory", otherReleasesDirectory,
					"--instance-groups-directory", someInstanceGroupsDirectory,
					"--instance-groups-directory", someOtherInstanceGroupsDirectory,
					"--jobs-directory", someJobsDirectory,
					"--jobs-directory", someOtherJobsDirectory,
					"--properties-directory", somePropertiesDirectory,
					"--runtime-configs-directory", someRuntimeConfigsDirectory,
					"--stemcell-tarball", stemcellTarball,
					"--stub-releases",
					"--variable", "some-variable=some-variable-value",
					"--version", "1.2.3",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0))

				archive, err := os.Open(outputFile)
				Expect(err).NotTo(HaveOccurred())

				archiveInfo, err := archive.Stat()
				Expect(err).NotTo(HaveOccurred())

				zr, err := zip.NewReader(archive, archiveInfo.Size())
				Expect(err).NotTo(HaveOccurred())

				seenFile := false
				for _, f := range zr.File {
					if f.Name == "embed/some-dir/some-nested-dir/some-file-to-embed" {
						seenFile = true
						r, err := f.Open()
						Expect(err).NotTo(HaveOccurred())

						content, err := ioutil.ReadAll(r)
						Expect(err).NotTo(HaveOccurred())

						Expect(content).To(Equal([]byte("content-of-some-file")))
					}
				}

				Expect(seenFile).To(BeTrue())
			})
		})
	})

	Context("when a --variables-file flag is provided", func() {
		var variableFile *os.File

		BeforeEach(func() {
			var err error
			variableFile, err = ioutil.TempFile(tmpDir, "variables-file")
			Expect(err).NotTo(HaveOccurred())
			defer variableFile.Close()

			variables := map[string]string{"some-variable": "some-variable-value"}
			data, err := yaml.Marshal(&variables)
			Expect(err).NotTo(HaveOccurred())

			n, err := variableFile.Write(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveLen(n))
		})

		It("interpolates variables from the file into the metadata", func() {
			command := exec.Command(pathToMain,
				"bake",
				"--forms-directory", someFormsDirectory,
				"--forms-directory", someOtherFormsDirectory,
				"--forms-directory", someFormsDirectory,
				"--forms-directory", someOtherFormsDirectory,
				"--icon", someIconPath,
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--metadata", metadata,
				"--migrations-directory", "fixtures/extra-migrations",
				"--migrations-directory", "fixtures/migrations",
				"--output-file", outputFile,
				"--properties-directory", somePropertiesDirectory,
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--instance-groups-directory", someOtherInstanceGroupsDirectory,
				"--jobs-directory", someJobsDirectory,
				"--jobs-directory", someOtherJobsDirectory,
				"--properties-directory", somePropertiesDirectory,
				"--runtime-configs-directory", someRuntimeConfigsDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--bosh-variables-directory", someVariablesDirectory,
				"--variables-file", variableFile.Name(),
				"--version", "1.2.3",
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			archive, err := os.Open(outputFile)
			Expect(err).NotTo(HaveOccurred())

			archiveInfo, err := archive.Stat()
			Expect(err).NotTo(HaveOccurred())

			zr, err := zip.NewReader(archive, archiveInfo.Size())
			Expect(err).NotTo(HaveOccurred())

			var file io.ReadCloser
			for _, f := range zr.File {
				if f.Name == "metadata/cool-product-name.yml" {
					file, err = f.Open()
					Expect(err).NotTo(HaveOccurred())
					break
				}
			}

			Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
			metadataContents, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(metadataContents)).To(ContainSubstring("custom_variable: some-variable-value"))
		})
	})

	Context("failure cases", func() {
		Context("when a release tarball does not exist", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain,
					"bake",
					"--forms-directory", someFormsDirectory,
					"--forms-directory", someOtherFormsDirectory,
					"--icon", someIconPath,
					"--metadata", "metadata.yml",
					"--output-file", outputFile,
					"--properties-directory", somePropertiesDirectory,
					"--releases-directory", "missing-directory",
					"--stemcell-tarball", "stemcell.tgz",
					"--version", "1.2.3",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("open missing-directory: no such file or directory"))
			})
		})

		Context("when the output directory is not writable", func() {
			It("prints an error and exit 1", func() {
				command := exec.Command(pathToMain,
					"bake",
					"--forms-directory", someFormsDirectory,
					"--forms-directory", someOtherFormsDirectory,
					"--icon", someIconPath,
					"--metadata", metadata,
					"--output-file", "/path/to/missing/dir/product.zip",
					"--releases-directory", someReleasesDirectory,
					"--releases-directory", otherReleasesDirectory,
					"--instance-groups-directory", someInstanceGroupsDirectory,
					"--instance-groups-directory", someOtherInstanceGroupsDirectory,
					"--jobs-directory", someJobsDirectory,
					"--jobs-directory", someOtherJobsDirectory,
					"--properties-directory", somePropertiesDirectory,
					"--runtime-configs-directory", someRuntimeConfigsDirectory,
					"--stemcell-tarball", stemcellTarball,
					"--variable", "some-variable=some-variable-value",
					"--version", "1.2.3",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(string(session.Err.Contents()))).To(ContainSubstring("no such file or directory"))
			})
		})
	})
})
