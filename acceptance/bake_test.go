package acceptance_test

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("bake command", func() {
	var (
		outputFile string
		tmpDir     string

		commandWithArgs []string
	)

	const (
		someIconPath                     = "fixtures/icon"
		somePropertiesDirectory          = "fixtures/properties"
		someReleasesDirectory            = "fixtures/releases"
		otherReleasesDirectory           = "fixtures/releases2"
		someRuntimeConfigsDirectory      = "fixtures/runtime-config"
		someBOSHVariablesDirectory       = "fixtures/bosh-vars"
		someFormsDirectory               = "fixtures/forms"
		someOtherFormsDirectory          = "fixtures/forms2"
		someInstanceGroupsDirectory      = "fixtures/instance-groups"
		someOtherInstanceGroupsDirectory = "fixtures/instance-groups2"
		someJobsDirectory                = "fixtures/jobs"
		someOtherJobsDirectory           = "fixtures/jobs2"
		variableFile                     = "fixtures/variables-file"
		someVarFile                      = "fixtures/var-dir/var-file.yml"
		someAssetsYMLPath                = "fixtures/assets.yml"
		cfSHA1                           = "b383f3177e4fc4f0386b7a06ddbc3f57e7dbf09f"
		diegoSHA1                        = "ade2a81b4bfda4eb7062cb1a9314f8941ae11d06"
		stemcellTarball                  = "fixtures/stemcell.tgz"
		metadata                         = "fixtures/metadata.yml"
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		tileDir, err := ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		outputFile = filepath.Join(tileDir, "cool-product-1.2.3-build.4.pivotal")

		commandWithArgs = []string{
			"bake",
			"--bosh-variables-directory", someBOSHVariablesDirectory,
			"--forms-directory", someFormsDirectory,
			"--forms-directory", someOtherFormsDirectory,
			"--icon", someIconPath,
			"--instance-groups-directory", someInstanceGroupsDirectory,
			"--instance-groups-directory", someOtherInstanceGroupsDirectory,
			"--jobs-directory", someJobsDirectory,
			"--jobs-directory", someOtherJobsDirectory,
			"--metadata", metadata,
			"--output-file", outputFile,
			"--properties-directory", somePropertiesDirectory,
			"--releases-directory", otherReleasesDirectory,
			"--releases-directory", someReleasesDirectory,
			"--runtime-configs-directory", someRuntimeConfigsDirectory,
			"--stemcell-tarball", stemcellTarball,
			"--variable", "some-variable=some-variable-value",
			"--variables-file", someVarFile,
			"--version", "1.2.3",
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("generates a tile with the correct metadata", func() {
		commandWithArgs = append(commandWithArgs, "--migrations-directory",
			"fixtures/extra-migrations",
			"--migrations-directory",
			"fixtures/migrations",
			"--variables-file",
			variableFile)

		command := exec.Command(pathToMain, commandWithArgs...)

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
			if f.Name == "metadata/metadata.yml" {
				file, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
				break
			}
		}

		Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
		metadataContents, err := ioutil.ReadAll(file)
		Expect(err).NotTo(HaveOccurred())

		renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
		Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))

		// Bosh Variables
		Expect(string(metadataContents)).To(ContainSubstring("name: variable-1"))
		Expect(string(metadataContents)).To(ContainSubstring("name: variable-2"))
		Expect(string(metadataContents)).To(ContainSubstring("type: certificate"))
		Expect(string(metadataContents)).To(ContainSubstring("some_option: Option value"))

		// Template Variables
		Expect(string(metadataContents)).To(ContainSubstring("custom_variable: some-variable-value"))

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

		Eventually(session.Err).Should(gbytes.Say("Reading release manifests"))
		Eventually(session.Err).Should(gbytes.Say("Reading stemcell manifest"))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Building %s", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding metadata/metadata.yml to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603041539_custom_buildpacks.js to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603071158_auth_enterprise_sso.js to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Err).ShouldNot(gbytes.Say(fmt.Sprintf("Adding releases/not-a-tarball.txt to %s...", outputFile)))
	})

	Context("when the --sha256 flag is provided", func() {
		BeforeEach(func() {
			commandWithArgs = append(commandWithArgs, "--sha256")
		})

		It("outputs a sha256 checksum of the file to stderr", func() {
			command := exec.Command(pathToMain, commandWithArgs...)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			Expect(session.Err).To(gbytes.Say(fmt.Sprintf("Calculating SHA256 checksum of %s...", outputFile)))
			Expect(session.Err).To(gbytes.Say("SHA256 checksum: [0-9a-f]{64}"))

			contents, err := ioutil.ReadFile(fmt.Sprintf("%s.sha256", outputFile))
			Expect(err).NotTo(HaveOccurred())

			re := regexp.MustCompile("SHA256 checksum: ([0-9a-f]{64})")
			expectedChecksum := re.FindStringSubmatch(string(session.Err.Contents()))[1]

			Expect(string(contents)).To(Equal(expectedChecksum))
		})
	})

	Context("when the --assets-file flag is provided", func() {

		It("generates a tile with the correct metadata including the stemcell criteria from the asset file", func() {
			commandWithArgs = []string{
				"bake",
				"--bosh-variables-directory", someBOSHVariablesDirectory,
				"--forms-directory", someFormsDirectory,
				"--forms-directory", someOtherFormsDirectory,
				"--icon", someIconPath,
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--instance-groups-directory", someOtherInstanceGroupsDirectory,
				"--jobs-directory", someJobsDirectory,
				"--jobs-directory", someOtherJobsDirectory,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--properties-directory", somePropertiesDirectory,
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--runtime-configs-directory", someRuntimeConfigsDirectory,
				"--variable", "some-variable=some-variable-value",
				"--variables-file", someVarFile,
				"--version", "1.2.3",
				"--assets-file", someAssetsYMLPath,
			}
			commandWithArgs = append(commandWithArgs, "--migrations-directory",
				"fixtures/extra-migrations",
				"--migrations-directory",
				"fixtures/migrations",
				"--variables-file",
				variableFile)

			command := exec.Command(pathToMain, commandWithArgs...)

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
				if f.Name == "metadata/metadata.yml" {
					file, err = f.Open()
					Expect(err).NotTo(HaveOccurred())
					break
				}
			}

			Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
			metadataContents, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
			Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))

			// Bosh Variables
			Expect(string(metadataContents)).To(ContainSubstring("name: variable-1"))
			Expect(string(metadataContents)).To(ContainSubstring("name: variable-2"))
			Expect(string(metadataContents)).To(ContainSubstring("type: certificate"))
			Expect(string(metadataContents)).To(ContainSubstring("some_option: Option value"))

			// Template Variables
			Expect(string(metadataContents)).To(ContainSubstring("custom_variable: some-variable-value"))

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

			Eventually(session.Err).Should(gbytes.Say("Reading release manifests"))
			Eventually(session.Err).Should(gbytes.Say("Reading stemcell criteria from assets.lock"))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Building %s", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding metadata/metadata.yml to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603041539_custom_buildpacks.js to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603071158_auth_enterprise_sso.js to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
			Eventually(session.Err).ShouldNot(gbytes.Say(fmt.Sprintf("Adding releases/not-a-tarball.txt to %s...", outputFile)))
		})

		Context("failure cases", func() {
			It("assets.lock does not exist", func() {
				commandWithArgs = []string{
					"bake",
					"--instance-groups-directory", someInstanceGroupsDirectory,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--releases-directory", someReleasesDirectory,
					"--assets-file", "non-existent-assets.yml",
				}

				command := exec.Command(pathToMain, commandWithArgs...)
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("non-existent-assets.lock: no such file or directory"))
			})
		})
		It("errors out when assets.lock cannot be unmarshalled", func() {
			commandWithArgs = []string{
				"bake",
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", someReleasesDirectory,
				"--assets-file", "fixtures/bad-assets.yml",
			}

			command := exec.Command(pathToMain, commandWithArgs...)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say("cannot unmarshal"))
		})
	})
	Context("when the --metadata-only flag is specified", func() {
		BeforeEach(func() {
			commandWithArgs = []string{
				"bake",
				"--bosh-variables-directory", someBOSHVariablesDirectory,
				"--forms-directory", someFormsDirectory,
				"--forms-directory", someOtherFormsDirectory,
				"--icon", someIconPath,
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--instance-groups-directory", someOtherInstanceGroupsDirectory,
				"--jobs-directory", someJobsDirectory,
				"--jobs-directory", someOtherJobsDirectory,
				"--metadata", metadata,
				"--metadata-only",
				"--properties-directory", somePropertiesDirectory,
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--runtime-configs-directory", someRuntimeConfigsDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--variable", "some-variable=some-variable-value",
				"--variables-file", someVarFile,
				"--version", "1.2.3",
			}
		})

		It("outputs the generated metadata to stdout", func() {
			command := exec.Command(pathToMain, commandWithArgs...)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
			Eventually(session.Out.Contents).Should(HelpfullyMatchYAML(renderedYAML))
		})
	})

	Context("when the --stub-releases flag is specified", func() {
		It("creates a tile with empty release tarballs", func() {
			commandWithArgs = append(commandWithArgs, "--stub-releases")

			command := exec.Command(pathToMain, commandWithArgs...)

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
			command := exec.Command(pathToMain, commandWithArgs...)

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
			var emptyMigrationsFolderModified time.Time
			for _, f := range zr.File {
				if f.Name == "migrations/v1/" {
					emptyMigrationsFolderMode = f.Mode()
					emptyMigrationsFolderModified = f.FileHeader.Modified
					break
				}
			}

			Expect(emptyMigrationsFolderMode.IsDir()).To(BeTrue())
			Expect(emptyMigrationsFolderModified).To(BeTemporally("~", time.Now(), time.Minute))

			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Creating empty migrations folder in %s...", outputFile)))
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

				commandWithArgs = append(commandWithArgs,
					"--embed", otherFileToEmbed,
					"--embed", someFileToEmbed,
					"--stub-releases")

				command := exec.Command(pathToMain, commandWithArgs...)

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

					Expect(f.FileHeader.Modified).To(BeTemporally("~", time.Now(), time.Minute))
				}

				Expect(seenSomeFile).To(BeTrue())
				Expect(seenOtherFile).To(BeTrue())
			})
		})

		Context("when an embed directory is specified", func() {
			It("embeds the root directory and retains its structure", func() {
				dirToAdd := filepath.Join(tmpDir, "some-dir")
				nestedDir := filepath.Join(dirToAdd, "some-nested-dir")
				someFileToEmbed := filepath.Join(nestedDir, "some-file-to-embed")

				err := os.MkdirAll(nestedDir, 0700)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(someFileToEmbed, []byte("content-of-some-file"), 0600)
				Expect(err).NotTo(HaveOccurred())

				commandWithArgs = append(commandWithArgs,
					"--embed", dirToAdd,
					"--stub-releases")
				command := exec.Command(pathToMain, commandWithArgs...)

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

	Context("failure cases", func() {
		Context("when a release tarball does not exist", func() {
			It("prints an error and exits 1", func() {
				commandWithArgs = append(commandWithArgs, "--releases-directory", "missing-directory")
				command := exec.Command(pathToMain, commandWithArgs...)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("lstat missing-directory: no such file or directory"))
			})
		})

		Context("when a assets file does not exist", func() {
			It("prints an error and exits 1", func() {
				commandWithArgs := []string{"bake",
					"--bosh-variables-directory", someBOSHVariablesDirectory,
					"--forms-directory", someFormsDirectory,
					"--forms-directory", someOtherFormsDirectory,
					"--icon", someIconPath,
					"--instance-groups-directory", someInstanceGroupsDirectory,
					"--instance-groups-directory", someOtherInstanceGroupsDirectory,
					"--jobs-directory", someJobsDirectory,
					"--jobs-directory", someOtherJobsDirectory,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--properties-directory", somePropertiesDirectory,
					"--releases-directory", otherReleasesDirectory,
					"--releases-directory", someReleasesDirectory,
					"--runtime-configs-directory", someRuntimeConfigsDirectory,
					"--assets-file", "non-existent-assets.yml",
					"--variable", "some-variable=some-variable-value",
					"--variables-file", someVarFile,
					"--version", "1.2.3",
				}
				command := exec.Command(pathToMain, commandWithArgs...)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("non-existent-assets.lock: no such file or directory"))
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
					"--bosh-variables-directory", someBOSHVariablesDirectory,
					"--variable", "some-variable=some-variable-value",
					"--variables-file", someVarFile,
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
