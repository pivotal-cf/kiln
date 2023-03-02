package acceptance_test

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var (
	pathToMain   string
	buildVersion string
)

func TestAcceptance(t *testing.T) {
	SetDefaultEventuallyTimeout(time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance")
}

var _ = BeforeSuite(func() {
	buildVersion = fmt.Sprintf("v0.0.0-dev.%d", time.Now().Unix())

	var err error
	pathToMain, err = gexec.Build("github.com/pivotal-cf/kiln",
		"--ldflags", fmt.Sprintf("-X main.version=%s", buildVersion))
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

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
		multiStemcellDirectory           = "fixtures/multiple-stemcells"
		singleStemcellDirectory          = "fixtures/single-stemcell"
		variableFile                     = "fixtures/variables-file"
		someVarFile                      = "fixtures/var-dir/var-file.yml"
		someKilnfilePath                 = "fixtures/Kilnfile"
		cfSHA1                           = "b383f3177e4fc4f0386b7a06ddbc3f57e7dbf09f"
		diegoSHA1                        = "ade2a81b4bfda4eb7062cb1a9314f8941ae11d06"
		metadata                         = "fixtures/metadata.yml"
		metadataWithStemcellCriteria     = "fixtures/metadata-with-stemcell-criteria.yml"
		metadataWithMultipleStemcells    = "fixtures/metadata-with-multiple-stemcells.yml"
		metadataWithStemcellTarball      = "fixtures/metadata-with-stemcell-tarball.yml"
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = os.MkdirTemp("", "kiln-main-test")
		Expect(err).NotTo(HaveOccurred())

		tileDir, err := os.MkdirTemp(tmpDir, "")
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
			"--variable", "some-variable=some-variable-value",
			"--variables-file", someVarFile,
			"--version", "1.2.3",
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("generates a tile with the correct metadata", func() {
		commandWithArgs = append(commandWithArgs,
			"--migrations-directory", "fixtures/extra-migrations",
			"--migrations-directory", "fixtures/migrations",
			"--variables-file", variableFile,
			"--stemcells-directory", singleStemcellDirectory,
		)

		command := exec.Command(pathToMain, commandWithArgs...)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		archive, err := os.Open(outputFile)
		Expect(err).NotTo(HaveOccurred())

		archiveInfo, err := archive.Stat()
		Expect(err).NotTo(HaveOccurred())

		bakedTile, err := zip.NewReader(archive, archiveInfo.Size())
		Expect(err).NotTo(HaveOccurred())

		file, err := bakedTile.Open("metadata/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		metadataContents, err := io.ReadAll(file)
		Expect(err).NotTo(HaveOccurred())

		renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
		Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))

		archivedMigration1, err := bakedTile.Open("migrations/v1/201603041539_custom_buildpacks.js")
		Expect(err).NotTo(HaveOccurred())
		archivedMigration2, err := bakedTile.Open("migrations/v1/201603071158_auth_enterprise_sso.js")
		Expect(err).NotTo(HaveOccurred())
		archivedMigration3, err := bakedTile.Open("migrations/v1/some_migration.js")
		Expect(err).NotTo(HaveOccurred())

		contents, err := io.ReadAll(archivedMigration1)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("custom-buildpack-migration\n"))

		contents, err = io.ReadAll(archivedMigration2)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("auth-enterprise-sso-migration\n"))

		contents, err = io.ReadAll(archivedMigration3)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("some_migration\n"))

		Eventually(session.Err).Should(gbytes.Say("Reading release manifests"))
		Eventually(session.Err).Should(gbytes.Say("Reading stemcells from directories"))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Building %s", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding metadata/metadata.yml to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603041539_custom_buildpacks.js to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603071158_auth_enterprise_sso.js to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Err).ShouldNot(gbytes.Say(fmt.Sprintf("Adding releases/not-a-tarball.txt to %s...", outputFile)))
	})

	Context("when multiple stemcells are provided", func() {
		BeforeEach(func() {
			commandWithArgs = []string{
				"bake",
				"--releases-directory", someReleasesDirectory,
				"--icon", someIconPath,
				"--metadata", metadataWithMultipleStemcells,
				"--stemcells-directory", multiStemcellDirectory,
				"--output-file", outputFile,
				"--version", "1.2.3",
			}
		})

		It("interpolates metadata file using multiple stemcells", func() {
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

			file, err := zr.Open("metadata/metadata.yml")
			Expect(err).NotTo(HaveOccurred())

			Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
			metadataContents, err := io.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			renderedYAML := fmt.Sprintf(expectedMetadataWithMultipleStemcells, cfSHA1)
			Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))
		})
	})

	Context("when the --stemcell-tarball flag is provided", func() {
		BeforeEach(func() {
			commandWithArgs = []string{
				"bake",
				"--releases-directory", someReleasesDirectory,
				"--icon", someIconPath,
				"--metadata", metadataWithStemcellTarball,
				"--stemcell-tarball", singleStemcellDirectory + "/stemcell.tgz",
				"--output-file", outputFile,
				"--version", "1.2.3",
			}
		})

		It("interpolates metadata file using a single stemcell", func() {
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

			file, err := zr.Open("metadata/metadata.yml")
			Expect(err).NotTo(HaveOccurred())

			Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
			metadataContents, err := io.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			renderedYAML := fmt.Sprintf(expectedMetadataWithStemcellTarball, cfSHA1)
			Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))
		})
	})

	Context("when the --sha256 flag is provided", func() {
		BeforeEach(func() {
			commandWithArgs = append(commandWithArgs,
				"--sha256",
				"--stemcells-directory", singleStemcellDirectory,
			)
		})

		It("outputs a sha256 checksum of the file to stderr", func() {
			command := exec.Command(pathToMain, commandWithArgs...)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			Expect(session.Err).To(gbytes.Say(fmt.Sprintf("Calculating SHA256 checksum of %s...", outputFile)))
			Expect(session.Err).To(gbytes.Say("SHA256 checksum: [0-9a-f]{64}"))

			contents, err := os.ReadFile(fmt.Sprintf("%s.sha256", outputFile))
			Expect(err).NotTo(HaveOccurred())

			re := regexp.MustCompile(`SHA256 checksum: ([0-9a-f]{64})`)
			expectedChecksum := re.FindStringSubmatch(string(session.Err.Contents()))[1]

			Expect(string(contents)).To(Equal(expectedChecksum))
		})
	})

	Context("when the --kilnfile flag is provided", func() {
		It("generates a tile with the correct metadata including the stemcell criteria from the Kilnfile.lock", func() {
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
				"--kilnfile", someKilnfilePath,
			}
			commandWithArgs = append(commandWithArgs,
				"--migrations-directory", "fixtures/extra-migrations",
				"--migrations-directory", "fixtures/migrations",
				"--variables-file", variableFile,
			)

			command := exec.Command(pathToMain, commandWithArgs...)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			bakedFilePtr, err := os.Open(outputFile)
			Expect(err).NotTo(HaveOccurred())

			archiveInfo, err := bakedFilePtr.Stat()
			Expect(err).NotTo(HaveOccurred())

			bakedTile, err := zip.NewReader(bakedFilePtr, archiveInfo.Size())
			Expect(err).NotTo(HaveOccurred())

			file, err := bakedTile.Open("metadata/metadata.yml")
			Expect(err).NotTo(HaveOccurred())

			Expect(file).NotTo(BeNil(), "metadata was not found in built tile")
			metadataContents, err := io.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
			Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))

			archivedMigration1, err := bakedTile.Open("migrations/v1/201603041539_custom_buildpacks.js")
			Expect(err).NotTo(HaveOccurred())
			archivedMigration2, err := bakedTile.Open("migrations/v1/201603071158_auth_enterprise_sso.js")
			Expect(err).NotTo(HaveOccurred())
			archivedMigration3, err := bakedTile.Open("migrations/v1/some_migration.js")
			Expect(err).NotTo(HaveOccurred())

			contents, err := io.ReadAll(archivedMigration1)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("custom-buildpack-migration\n"))

			contents, err = io.ReadAll(archivedMigration2)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("auth-enterprise-sso-migration\n"))

			contents, err = io.ReadAll(archivedMigration3)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("some_migration\n"))

			Eventually(session.Err).Should(gbytes.Say("Reading release manifests"))
			Eventually(session.Err).Should(gbytes.Say("Reading stemcell criteria from Kilnfile.lock"))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Building %s", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding metadata/metadata.yml to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603041539_custom_buildpacks.js to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding migrations/v1/201603071158_auth_enterprise_sso.js to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
			Eventually(session.Err).ShouldNot(gbytes.Say(fmt.Sprintf("Adding releases/not-a-tarball.txt to %s...", outputFile)))
		})

		Context("failure cases", func() {
			It("Kilnfile.lock does not exist", func() {
				commandWithArgs = []string{
					"bake",
					"--instance-groups-directory", someInstanceGroupsDirectory,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--releases-directory", someReleasesDirectory,
					"--kilnfile", "non-existent-kilnfile",
				}

				command := exec.Command(pathToMain, commandWithArgs...)
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("non-existent-kilnfile.lock: no such file or directory"))
			})
		})
		It("errors out when Kilnfile.lock cannot be unmarshalled", func() {
			commandWithArgs = []string{
				"bake",
				"--instance-groups-directory", someInstanceGroupsDirectory,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", someReleasesDirectory,
				"--kilnfile", "fixtures/bad-Kilnfile",
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
				"--stemcells-directory", singleStemcellDirectory,
				"--variable", "some-variable=some-variable-value",
				"--variables-file", someVarFile,
				"--version", "1.2.3",
			}
		})

		It("outputs the generated metadata to stdout", func() {
			command := exec.Command(pathToMain, commandWithArgs...)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// intervals added to make tests pass. it is taking too long locally
			Eventually(session, time.Second*10).Should(gexec.Exit(0))

			renderedYAML := fmt.Sprintf(expectedMetadata, diegoSHA1, cfSHA1)
			Eventually(session.Out.Contents).Should(HelpfullyMatchYAML(renderedYAML))
		})
	})

	Context("when the --stub-releases flag is specified", func() {
		It("creates a tile with empty release tarballs", func() {
			commandWithArgs = append(commandWithArgs,
				"--stemcells-directory", singleStemcellDirectory,
				"--stub-releases",
			)

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
				if path.Dir(f.Name) != "releases" {
					continue
				}
				Expect(f.UncompressedSize64).To(Equal(uint64(0)))
			}
		})
	})

	Context("when no migrations are provided", func() {
		It("creates empty migrations folder", func() {
			commandWithArgs = append(commandWithArgs,
				"--stemcells-directory", singleStemcellDirectory,
			)

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

				err := os.WriteFile(someFileToEmbed, []byte("content-of-some-file"), 0o600)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(otherFileToEmbed, []byte("content-of-other-file"), 0o755)
				Expect(err).NotTo(HaveOccurred())

				commandWithArgs = append(commandWithArgs,
					"--embed", otherFileToEmbed,
					"--embed", someFileToEmbed,
					"--stub-releases",
					"--stemcells-directory", singleStemcellDirectory,
				)

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

						content, err := io.ReadAll(r)
						Expect(err).NotTo(HaveOccurred())

						Expect(content).To(Equal([]byte("content-of-some-file")))
					}

					if f.Name == "embed/other-file-to-embed" {
						seenOtherFile = true
						r, err := f.Open()
						Expect(err).NotTo(HaveOccurred())

						content, err := io.ReadAll(r)
						Expect(err).NotTo(HaveOccurred())

						mode := f.FileHeader.Mode()
						Expect(mode).To(Equal(os.FileMode(0o755)))

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

				err := os.MkdirAll(nestedDir, 0o700)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(someFileToEmbed, []byte("content-of-some-file"), 0o600)
				Expect(err).NotTo(HaveOccurred())

				commandWithArgs = append(commandWithArgs,
					"--embed", dirToAdd,
					"--stub-releases",
					"--stemcells-directory", singleStemcellDirectory,
				)
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

						content, err := io.ReadAll(r)
						Expect(err).NotTo(HaveOccurred())

						Expect(content).To(Equal([]byte("content-of-some-file")))
					}
				}

				Expect(seenFile).To(BeTrue())
			})
		})
	})

	Context("when neither --kilnfile nor --stemcells-directory are provided", func() {
		It("generates a tile with unchanged stemcell criteria", func() {
			commandWithArgs = []string{
				"bake",
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--icon", someIconPath,
				"--metadata", metadataWithStemcellCriteria,
				"--output-file", outputFile,
				"--version", "1.2.3",
			}

			command := exec.Command(pathToMain, commandWithArgs...)
			fmt.Println(commandWithArgs)

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
			metadataContents, err := io.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			renderedYAML := fmt.Sprintf(expectedMetadataWithStemcellCriteria, diegoSHA1, cfSHA1)
			Expect(metadataContents).To(HelpfullyMatchYAML(renderedYAML))

			Eventually(session.Err).Should(gbytes.Say("Reading release manifests"))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Building %s", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding metadata/metadata.yml to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Creating empty migrations folder in %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
			Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
			Eventually(session.Err).ShouldNot(gbytes.Say(fmt.Sprintf("Adding releases/not-a-tarball.txt to %s...", outputFile)))
		})
	})

	Context("failure cases", func() {
		Context("when a release tarball does not exist", func() {
			XIt("prints an error and exits 1", func() {
				commandWithArgs = append(commandWithArgs, "--releases-directory", "missing-directory")
				command := exec.Command(pathToMain, commandWithArgs...)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				// TODO: this expectation seems incorrect, but it's what the test was doing before
				//Expect(string(session.Err.Contents())).To(ContainSubstring("lstat missing-directory: no such file or directory"))
			})
		})

		Context("when a Kilnfile.lock does not exist", func() {
			It("prints an error and exits 1", func() {
				commandWithArgs := []string{
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
					"--kilnfile", "non-existent-Kilnfile",
					"--variable", "some-variable=some-variable-value",
					"--variables-file", someVarFile,
					"--version", "1.2.3",
				}
				command := exec.Command(pathToMain, commandWithArgs...)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("non-existent-Kilnfile.lock: no such file or directory"))
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
					"--stemcells-directory", singleStemcellDirectory,
					"--bosh-variables-directory", someBOSHVariablesDirectory,
					"--variable", "some-variable=some-variable-value",
					"--variables-file", someVarFile,
					"--version", "1.2.3",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("no such file or directory"))
			})
		})
	})
})

var expectedMetadata = `---
description: this is the description
some_forms:
- description: some-other-form-description
  label: some-other-form-label
  name: some-other-config
- description: some-form-description
  label: some-form-label
  name: some-config
- description: some-form-description
  label: some-form-label
  name: some-more-config
icon_img: aS1hbS1zb21lLWltYWdl
install_time_verifiers:
- name: Verifiers::SsoUrlVerifier
  properties:
    url: .properties.uaa.saml.sso_url
some_job_types:
- label: Some Instance Group
  name: some-instance-group
  templates:
  - name: some-job
    release: some-release
- label: Some Other Instance Group
  name: some-other-instance-group
  templates:
  - name: some-other-job
    release: some-other-release
label: Pivotal Elastic Runtime
metadata_version: "1.7"
minimum_version_for_upgrade: 1.6.9-build.0
custom_variable: some-variable-value
literal_variable: |
  { "some": "value" }
boolean_variable: true
some_bosh_variables:
- name: variable-1
  type: certificate
  options:
    some_option: Option value
- name: variable-2
  type: password
name: cool-product-name
post_deploy_errands:
- name: smoke-tests
product_version: 1.2.3
some_property_blueprints:
- name: some_templated_property_blueprint
  type: boolean
  configurable: false
  default: true
provides_product_versions:
- name: cf
  version: 1.7.0.0
rank: 90
some_releases:
- file: diego-release-0.1467.1-3215.4.0.tgz
  name: diego
  version: 0.1467.1
  sha1: %s
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
some_stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
some_runtime_configs:
- name: some-runtime-config
  runtime_config: |
    releases:
    - name: some-addon
      version: some-addon-version
serial: false
selected_value: "235"
`

var expectedMetadataWithStemcellCriteria = `---
icon_img: aS1hbS1zb21lLWltYWdl
label: Pivotal Elastic Runtime
metadata_version: "1.7"
name: cool-product-name
product_version: 1.2.3
some_releases:
- file: diego-release-0.1467.1-3215.4.0.tgz
  name: diego
  version: 0.1467.1
  sha1: %s
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
stemcell_criteria:
  os: ubuntu-xenial
  version: 250.21
  requires_cpi: false
  enable_patch_security_updates: true
`

var expectedMetadataWithMultipleStemcells = `---
icon_img: aS1hbS1zb21lLWltYWdl
label: Pivotal Elastic Runtime
metadata_version: "1.7"
name: cool-product-name
product_version: 1.2.3
some_releases:
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
additional_stemcells_criteria:
- os: windows
  version: "2019.4"
`

var expectedMetadataWithStemcellTarball = `---
icon_img: aS1hbS1zb21lLWltYWdl
label: Pivotal Elastic Runtime
metadata_version: "1.7"
name: cool-product-name
product_version: 1.2.3
some_releases:
- file: cf-release-235.0.0-3215.4.0.tgz
  name: cf
  version: "235"
  sha1: %s
stemcell_criteria:
  os: ubuntu-trusty
  version: "3215.4"
`
