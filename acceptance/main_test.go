package acceptance

import (
	"archive/zip"
	"encoding/base64"
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
		metadata                    string
		otherReleasesDirectory      string
		outputFile                  string
		someIconPath                string
		someReleasesDirectory       string
		someRuntimeConfigsDirectory string
		someVariablesDirectory      string
		someFormDirectory           string
		stemcellTarball             string
		tmpDir                      string
	)

	BeforeEach(func() {
		var err error

		tmpDir, err := ioutil.TempDir("", "kiln-main-test")
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

		someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someRuntimeConfigsDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someVariablesDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		someFormDirectory, err = ioutil.TempDir(tmpDir, "")
		Expect(err).NotTo(HaveOccurred())

		cfReleaseManifest := `---
name: cf
version: 235
`

		_, err = ioutil.TempFile(someReleasesDirectory, "")
		Expect(err).NotTo(HaveOccurred())

		_, err = createTarball(someReleasesDirectory, "cf-release-235.0.0-3215.4.0.tgz", "release.MF", cfReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		diegoReleaseManifest := `---
name: diego
version: 0.1467.1
key: value
`

		_, err = createTarball(otherReleasesDirectory, "diego-release-0.1467.1-3215.4.0.tgz", "release.MF", diegoReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		notATarball := filepath.Join(someReleasesDirectory, "not-a-tarball.txt")
		_ = ioutil.WriteFile(notATarball, []byte(`this is not a tarball`), 0644)
		stemcellManifest := `---
version: "3215.4"
operating_system: ubuntu-trusty
`

		stemcellTarball, err = createTarball(tmpDir, "stemcell.tgz", "stemcell.MF", stemcellManifest)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someFormDirectory, "_order.yml"), []byte(`---
forms:
  - some-other-config
  - some-config
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someFormDirectory, "some-config.yml"), []byte(`---
form:
  name: some-config
  label: some-form-label
  description: some-form-description
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(someFormDirectory, "some-other-config.yml"), []byte(`---
form:
  name: some-other-config
  label: some-other-form-label
  description: some-other-form-description
`), 0644)

		Expect(err).NotTo(HaveOccurred())
		err = ioutil.WriteFile(filepath.Join(someRuntimeConfigsDirectory, "some-addon.yml"), []byte(`---
runtime_configs:
- name: some_addon
  runtime_config: |
    releases:
    - name: some-addon
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
		err = ioutil.WriteFile(metadata, []byte(`---
name: cool-product-name
metadata_version: '1.7'
provides_product_versions:
- name: cf
  version: 1.7.0.0
product_version: &product_version "1.7.0.0$PRERELEASE_VERSION$"
minimum_version_for_upgrade: 1.6.9-build.0
label: Pivotal Elastic Runtime
description:
  this is the description
icon_image: unused-icon-image
rank: 90
serial: false
install_time_verifiers:
- name: Verifiers::SsoUrlVerifier
  properties:
    url: .properties.uaa.saml.sso_url
post_deploy_errands:
- name: smoke-tests
form_types:
- name: domains
  label: Domains
job_types:
- name: consul_server
  label: Consul
property_blueprints:
- name: product_version
  type: string
  configurable: false
  default: *product_version
`), 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("generates metadata that includes all the correct information", func() {
		command := exec.Command(pathToMain,
			"bake",
			"--icon", someIconPath,
			"--metadata", metadata,
			"--output-file", outputFile,
			"--releases-directory", otherReleasesDirectory,
			"--releases-directory", someReleasesDirectory,
			"--runtime-configs-directory", someRuntimeConfigsDirectory,
			"--stemcell-tarball", stemcellTarball,
			"--variables-directory", someVariablesDirectory,
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
		contents, err := ioutil.ReadAll(file)
		Expect(err).NotTo(HaveOccurred())

		iconData := base64.StdEncoding.EncodeToString([]byte("i-am-some-image"))
		expectedYaml := fmt.Sprintf(`---
name: cool-product-name
stemcell_criteria:
  os: ubuntu-trusty
  requires_cpi: false
  version: "3215.4"
releases:
- name: diego
  file: diego-release-0.1467.1-3215.4.0.tgz
  version: 0.1467.1
- name: cf
  file: cf-release-235.0.0-3215.4.0.tgz
  version: "235"
metadata_version: '1.7'
provides_product_versions:
- name: cf
  version: 1.7.0.0
product_version: "1.2.3"
minimum_version_for_upgrade: 1.6.9-build.0
label: Pivotal Elastic Runtime
description:
  this is the description
icon_image: %s
rank: 90
serial: false
install_time_verifiers:
- name: Verifiers::SsoUrlVerifier
  properties:
    url: .properties.uaa.saml.sso_url
post_deploy_errands:
- name: smoke-tests
form_types:
- name: domains
  label: Domains
job_types:
- name: consul_server
  label: Consul
property_blueprints:
- name: product_version
  type: string
  configurable: false
  default: "1.2.3"
runtime_configs:
- name: some_addon
  runtime_config: |
    releases:
    - name: some-addon
variables:
- name: variable-1
  type: certificate
  options:
    some_option: Option value
- name: variable-2
  type: password
`, iconData)

		Expect(contents).To(MatchYAML(expectedYaml))
	})

	Context("when the --form-directory flag is provided", func() {
		It("merges from the given directory into the metadata under the form_types key", func() {
			command := exec.Command(pathToMain,
				"bake",
				"--form-directory", someFormDirectory,
				"--icon", someIconPath,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", otherReleasesDirectory,
				"--releases-directory", someReleasesDirectory,
				"--runtime-configs-directory", someRuntimeConfigsDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--variables-directory", someVariablesDirectory,
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
			contents, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			actualMetadata := map[string]interface{}{}
			err = yaml.Unmarshal(contents, actualMetadata)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualMetadata["form_types"]).To(Equal([]interface{}{
				map[interface{}]interface{}{
					"name":        "some-other-config",
					"label":       "some-other-form-label",
					"description": "some-other-form-description",
				},
				map[interface{}]interface{}{
					"name":        "some-config",
					"label":       "some-form-label",
					"description": "some-form-description",
				},
			}))
		})
	})

	It("copies the migrations to the migrations/v1 directory", func() {
		command := exec.Command(pathToMain,
			"bake",
			"--icon", someIconPath,
			"--metadata", metadata,
			"--migrations-directory", "fixtures/extra-migrations",
			"--migrations-directory", "fixtures/migrations",
			"--output-file", outputFile,
			"--releases-directory", someReleasesDirectory,
			"--stemcell-tarball", stemcellTarball,
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
	})

	It("logs the progress to stdout", func() {
		command := exec.Command(pathToMain,
			"bake",
			"--icon", someIconPath,
			"--metadata", metadata,
			"--migrations-directory", "fixtures/migrations",
			"--output-file", outputFile,
			"--releases-directory", otherReleasesDirectory,
			"--releases-directory", someReleasesDirectory,
			"--stemcell-tarball", stemcellTarball,
			"--version", "1.2.3",
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Creating metadata for %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say("Read manifest for release cf"))
		Eventually(session.Out).Should(gbytes.Say("Read manifest for stemcell version 3215.4"))
		Eventually(session.Out).Should(gbytes.Say("Injecting version \"1.2.3\" into metadata..."))
		Eventually(session.Out).Should(gbytes.Say("Read metadata"))
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
				"--icon", someIconPath,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", someReleasesDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--stub-releases",
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
				"--icon", someIconPath,
				"--metadata", metadata,
				"--output-file", outputFile,
				"--releases-directory", someReleasesDirectory,
				"--stemcell-tarball", stemcellTarball,
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
					"--embed", otherFileToEmbed,
					"--embed", someFileToEmbed,
					"--icon", someIconPath,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--releases-directory", someReleasesDirectory,
					"--stemcell-tarball", stemcellTarball,
					"--stub-releases",
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
					"--embed", dirToAdd,
					"--icon", someIconPath,
					"--metadata", metadata,
					"--output-file", outputFile,
					"--releases-directory", someReleasesDirectory,
					"--stemcell-tarball", stemcellTarball,
					"--stub-releases",
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

	Context("failure cases", func() {
		Context("when a release tarball does not exist", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain,
					"bake",
					"--icon", someIconPath,
					"--metadata", "metadata.yml",
					"--output-file", outputFile,
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
					"--icon", someIconPath,
					"--metadata", metadata,
					"--output-file", "/path/to/missing/dir/product.zip",
					"--releases-directory", someReleasesDirectory,
					"--stemcell-tarball", stemcellTarball,
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
