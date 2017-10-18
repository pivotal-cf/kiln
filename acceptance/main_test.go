package acceptance

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("kiln", func() {
	var (
		tempDir                string
		someReleasesDirectory  string
		otherReleasesDirectory string
		stemcellTarball        string
		metadata               string
		outputFile             string
	)

	BeforeEach(func() {
		var err error
		tileDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		outputFile = filepath.Join(tileDir, "cool-product-1.2.3-build.4.pivotal")

		someReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		otherReleasesDirectory, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		tempDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cfReleaseManifest := `---
name: cf
version: 235
`

		_, err = createTarball(someReleasesDirectory, "cf-release-235.0.0-3215.4.0.tgz", "release.MF", cfReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		diegoReleaseManifest := `---
name: diego
version: 0.1467.1
key: value
`

		_, err = createTarball(otherReleasesDirectory, "diego-release-0.1467.1-3215.4.0.tgz", "release.MF", diegoReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		stemcellManifest := `---
version: "3215.4"
operating_system: ubuntu-trusty
`

		stemcellTarball, err = createTarball(tempDir, "stemcell.tgz", "stemcell.MF", stemcellManifest)
		Expect(err).NotTo(HaveOccurred())

		metadata = filepath.Join(tempDir, "metadata.yml")
		err = ioutil.WriteFile(metadata, []byte(`---
metadata_version: '1.7'
provides_product_versions:
- name: cf
  version: 1.7.0.0
product_version: &product_version "1.7.0.0$PRERELEASE_VERSION$"
minimum_version_for_upgrade: 1.6.9-build.0
label: Pivotal Elastic Runtime
description:
  this is the description
icon_image: some-image
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

	It("generates a manifest that includes all the correct metadata", func() {
		command := exec.Command(pathToMain,
			"bake",
			"--stemcell-tarball", stemcellTarball,
			"--releases-directory", someReleasesDirectory,
			"--releases-directory", otherReleasesDirectory,
			"--metadata", metadata,
			"--version", "1.2.3",
			"--product-name", "cool-product-name",
			"--output-file", outputFile,
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

		contents, err := ioutil.ReadAll(file)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(MatchYAML(`---
name: cool-product-name
stemcell_criteria:
  os: ubuntu-trusty
  requires_cpi: false
  version: "3215.4"
releases:
- name: cf
  file: cf-release-235.0.0-3215.4.0.tgz
  version: "235"
- name: diego
  file: diego-release-0.1467.1-3215.4.0.tgz
  version: 0.1467.1
metadata_version: '1.7'
provides_product_versions:
- name: cf
  version: 1.7.0.0
product_version: "1.2.3"
minimum_version_for_upgrade: 1.6.9-build.0
label: Pivotal Elastic Runtime
description:
  this is the description
icon_image: some-image
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
`))
	})

	It("copies the migrations to the migrations/v1 directory", func() {
		command := exec.Command(pathToMain,
			"bake",
			"--releases-directory", someReleasesDirectory,
			"--stemcell-tarball", stemcellTarball,
			"--metadata", metadata,
			"--migrations-directory", "fixtures/extra-migrations",
			"--migrations-directory", "fixtures/migrations",
			"--version", "1.2.3",
			"--product-name", "cool-product-name",
			"--output-file", outputFile,
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
			"--releases-directory", someReleasesDirectory,
			"--releases-directory", otherReleasesDirectory,
			"--stemcell-tarball", stemcellTarball,
			"--metadata", metadata,
			"--migrations-directory", "fixtures/migrations",
			"--version", "1.2.3",
			"--product-name", "cool-product-name",
			"--output-file", outputFile,
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
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding releases/cf-release-235.0.0-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Calculating md5 sum of %s...", outputFile)))
		Eventually(session.Out).Should(gbytes.Say("Calculated md5 sum: [0-9a-f]{32}"))
	})

	Context("when the --stub-releases flag is specified", func() {
		It("creates a tile with empty release tarballs", func() {
			command := exec.Command(pathToMain,
				"bake",
				"--releases-directory", someReleasesDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--metadata", metadata,
				"--version", "1.2.3",
				"--stub-releases",
				"--product-name", "cool-product-name",
				"--output-file", outputFile,
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
				"--releases-directory", someReleasesDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--metadata", metadata,
				"--version", "1.2.3",
				"--product-name", "cool-product-name",
				"--output-file", outputFile,
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

	Context("when the metadata defines a runtime config", func() {
		It("generates a manifest that specifies the runtime config release version", func() {
			err := ioutil.WriteFile(metadata, []byte(`---
runtime_configs:
  - name: MY-RUNTIME-CONFIG
    runtime_config: |
      releases:
      - name: cf
      addons:
      - name: MY-ADDON-NAME
        jobs:
        - name: MY-RUNTIME-CONFIG-JOB
          release: cf
`), 0644)
			Expect(err).NotTo(HaveOccurred())

			command := exec.Command(pathToMain,
				"bake",
				"--stemcell-tarball", stemcellTarball,
				"--releases-directory", someReleasesDirectory,
				"--releases-directory", otherReleasesDirectory,
				"--metadata", metadata,
				"--version", "1.2.3",
				"--product-name", "cool-product-name",
				"--output-file", outputFile,
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

			contents, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(MatchYAML(`---
name: cool-product-name
stemcell_criteria:
  os: ubuntu-trusty
  requires_cpi: false
  version: "3215.4"
releases:
- name: cf
  file: cf-release-235.0.0-3215.4.0.tgz
  version: "235"
- name: diego
  file: diego-release-0.1467.1-3215.4.0.tgz
  version: 0.1467.1
runtime_configs:
  - name: MY-RUNTIME-CONFIG
    runtime_config: |
      releases:
      - name: cf
        version: "235"
      addons:
      - jobs:
        - name: MY-RUNTIME-CONFIG-JOB
          release: cf
        name: MY-ADDON-NAME
`))
		})

	})

	Context("when the --embed flag is specified", func() {
		It("creates a tile with the specified file copied into the embed directory", func() {
			someFileToEmbed := filepath.Join(tempDir, "some-file-to-embed")
			otherFileToEmbed := filepath.Join(tempDir, "other-file-to-embed")
			err := ioutil.WriteFile(someFileToEmbed, []byte("content-of-some-file"), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(otherFileToEmbed, []byte("content-of-other-file"), 0600)
			Expect(err).NotTo(HaveOccurred())

			command := exec.Command(pathToMain,
				"bake",
				"--releases-directory", someReleasesDirectory,
				"--stemcell-tarball", stemcellTarball,
				"--metadata", metadata,
				"--version", "1.2.3",
				"--stub-releases",
				"--product-name", "cool-product-name",
				"--embed", someFileToEmbed,
				"--embed", otherFileToEmbed,
				"--output-file", outputFile,
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

					Expect(content).To(Equal([]byte("content-of-other-file")))
				}
			}
			Expect(seenSomeFile).To(BeTrue())
			Expect(seenOtherFile).To(BeTrue())
		})
	})

	Context("failure cases", func() {
		Context("when a release tarball does not exist", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain,
					"bake",
					"--releases-directory", "missing-directory",
					"--metadata", "metadata.yml",
					"--stemcell-tarball", "stemcell.tgz",
					"--version", "1.2.3",
					"--product-name", "cool-product-name",
					"--output-file", outputFile,
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
					"--releases-directory", someReleasesDirectory,
					"--stemcell-tarball", stemcellTarball,
					"--metadata", metadata,
					"--version", "1.2.3",
					"--product-name", "cool-product-name",
					"--output-file", "/path/to/missing/dir/product.zip",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(string(session.Err.Contents()))).To(ContainSubstring("no such file or directory"))
			})
		})
	})
})
