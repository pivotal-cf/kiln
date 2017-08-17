package acceptance

import (
	"archive/zip"
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
		tileDir              string
		tempDir              string
		cfReleaseTarball     string
		diegoReleaseTarball  string
		stemcellTarball      string
		migration1           string
		migration2           string
		handcraft            string
		baseContentMigration string
		contentMigration     string
	)

	BeforeEach(func() {
		var err error
		tileDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		tempDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		cfReleaseManifest := `---
name: cf
version: 235
`

		cfReleaseTarball, err = createTarball(tempDir, "cf-release-235.0.0-3215.4.0.tgz", "release.MF", cfReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		diegoReleaseManifest := `---
name: diego
version: 0.1467.1
key: value
`

		diegoReleaseTarball, err = createTarball(tempDir, "diego-release-0.1467.1-3215.4.0.tgz", "release.MF", diegoReleaseManifest)
		Expect(err).NotTo(HaveOccurred())

		stemcellManifest := `---
version: "3215.4"
operating_system: ubuntu-trusty
`

		stemcellTarball, err = createTarball(tempDir, "stemcell.tgz", "stemcell.MF", stemcellManifest)
		Expect(err).NotTo(HaveOccurred())

		migration1 = filepath.Join(tempDir, "migration-1.js")
		err = ioutil.WriteFile(migration1, []byte("migration-1"), 0644)
		Expect(err).NotTo(HaveOccurred())

		migration2 = filepath.Join(tempDir, "migration-2.js")
		err = ioutil.WriteFile(migration2, []byte("migration-2"), 0644)
		Expect(err).NotTo(HaveOccurred())

		handcraft = filepath.Join(tempDir, "handcraft.yml")
		err = ioutil.WriteFile(handcraft, []byte(`---
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

		baseContentMigrationContents := `---
product: my-product
installation_schema_version: "1.6"
to_version: "7.8.9.0$PRERELEASE_VERSION$"
migrations: []`

		baseContentMigration = filepath.Join(tempDir, "base.yml")
		err = ioutil.WriteFile(baseContentMigration, []byte(baseContentMigrationContents), 0644)
		Expect(err).NotTo(HaveOccurred())

		contentMigrationContents := `---
from_version: 1.6.0-build.315
rules:
  - type: update
    selector: "product_version"
    to: "7.8.9.0$PRERELEASE_VERSION$"`
		contentMigration = filepath.Join(tempDir, "content_migration.yml")
		err = ioutil.WriteFile(contentMigration, []byte(contentMigrationContents), 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	It("generates a manifest that includes all the correct metadata", func() {
		command := exec.Command(pathToMain,
			"--stemcell-tarball", stemcellTarball,
			"--release-tarball", cfReleaseTarball,
			"--release-tarball", diegoReleaseTarball,
			"--handcraft", handcraft,
			"--version", "1.2.3-build.4",
			"--final-version", "1.2.3",
			"--product-name", "cool-product-name",
			"--filename-prefix", "cool-product",
			"--output-dir", tileDir)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		archive, err := os.Open(filepath.Join(tileDir, "cool-product-1.2.3-build.4.pivotal"))
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

	It("copies the tarballs to the releases directory", func() {
		command := exec.Command(pathToMain,
			"--release-tarball", cfReleaseTarball,
			"--release-tarball", diegoReleaseTarball,
			"--stemcell-tarball", stemcellTarball,
			"--handcraft", handcraft,
			"--version", "4.5.6-build.4",
			"--final-version", "4.5.6",
			"--product-name", "cool-product-name",
			"--filename-prefix", "cool-product",
			"--output-dir", tileDir)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		archive, err := os.Open(filepath.Join(tileDir, "cool-product-4.5.6-build.4.pivotal"))
		Expect(err).NotTo(HaveOccurred())

		archiveInfo, err := archive.Stat()
		Expect(err).NotTo(HaveOccurred())

		zr, err := zip.NewReader(archive, archiveInfo.Size())
		Expect(err).NotTo(HaveOccurred())

		archivedCFReleaseTarball, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		archivedDiegoReleaseTarball, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		for _, f := range zr.File {
			if f.Name == "releases/cf-release-235.0.0-3215.4.0.tgz" {
				file, err := f.Open()
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Copy(archivedCFReleaseTarball, file)
				Expect(err).NotTo(HaveOccurred())
			}

			if f.Name == "releases/diego-release-0.1467.1-3215.4.0.tgz" {
				file, err := f.Open()
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Copy(archivedDiegoReleaseTarball, file)
				Expect(err).NotTo(HaveOccurred())
			}
		}

		Expect(archivedCFReleaseTarball.Name()).To(MatchSHASumOf(cfReleaseTarball))
		Expect(archivedDiegoReleaseTarball.Name()).To(MatchSHASumOf(diegoReleaseTarball))
	})

	It("copies the migrations to the migrations/v1 directory", func() {
		command := exec.Command(pathToMain,
			"--release-tarball", cfReleaseTarball,
			"--stemcell-tarball", stemcellTarball,
			"--handcraft", handcraft,
			"--migration", migration1,
			"--migration", migration2,
			"--version", "7.8.9-build.4",
			"--final-version", "7.8.9",
			"--product-name", "cool-product-name",
			"--filename-prefix", "cool-product",
			"--output-dir", tileDir)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		archive, err := os.Open(filepath.Join(tileDir, "cool-product-7.8.9-build.4.pivotal"))
		Expect(err).NotTo(HaveOccurred())

		archiveInfo, err := archive.Stat()
		Expect(err).NotTo(HaveOccurred())

		zr, err := zip.NewReader(archive, archiveInfo.Size())
		Expect(err).NotTo(HaveOccurred())

		var (
			archivedMigration1 io.ReadCloser
			archivedMigration2 io.ReadCloser
		)

		for _, f := range zr.File {
			if f.Name == "migrations/v1/migration-1.js" {
				archivedMigration1, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
			}

			if f.Name == "migrations/v1/migration-2.js" {
				archivedMigration2, err = f.Open()
				Expect(err).NotTo(HaveOccurred())
			}
		}

		contents, err := ioutil.ReadAll(archivedMigration1)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(Equal([]byte("migration-1")))

		contents, err = ioutil.ReadAll(archivedMigration2)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(Equal([]byte("migration-2")))
	})

	It("logs the progress to stdout", func() {
		command := exec.Command(pathToMain,
			"--release-tarball", cfReleaseTarball,
			"--release-tarball", diegoReleaseTarball,
			"--stemcell-tarball", stemcellTarball,
			"--handcraft", handcraft,
			"--migration", migration1,
			"--migration", migration2,
			"--version", "3.2.1-build.4",
			"--final-version", "3.2.1",
			"--product-name", "cool-product-name",
			"--filename-prefix", "cool-product",
			"--output-dir", tileDir)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		Eventually(session.Out).Should(gbytes.Say("Creating metadata for .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Read manifest for release cf"))
		Eventually(session.Out).Should(gbytes.Say("Read manifest for stemcell version 3215.4"))
		Eventually(session.Out).Should(gbytes.Say("Injecting version \"3.2.1\" into handcraft..."))
		Eventually(session.Out).Should(gbytes.Say("Read handcraft"))
		Eventually(session.Out).Should(gbytes.Say("Marshaling metadata file..."))
		Eventually(session.Out).Should(gbytes.Say("Building .pivotal file..."))
		Eventually(session.Out).Should(gbytes.Say("Adding metadata/cool-product-name.yml to .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Adding migrations/v1/migration-1.js to .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Adding migrations/v1/migration-2.js to .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Adding releases/cf-release-235.0.0-3215.4.0.tgz to .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Adding releases/diego-release-0.1467.1-3215.4.0.tgz to .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Calculating md5 sum of .pivotal..."))
		Eventually(session.Out).Should(gbytes.Say("Calculated md5 sum: [0-9a-f]{32}"))
	})

	Context("when the --stub-releases flag is specified", func() {
		It("creates a tile with empty release tarballs", func() {
			command := exec.Command(pathToMain,
				"--release-tarball", cfReleaseTarball,
				"--release-tarball", diegoReleaseTarball,
				"--stemcell-tarball", stemcellTarball,
				"--handcraft", handcraft,
				"--version", "4.5.6-build.4",
				"--final-version", "4.5.6",
				"--stub-releases",
				"--product-name", "cool-product-name",
				"--filename-prefix", "cool-product",
				"--output-dir", tileDir)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			archive, err := os.Open(filepath.Join(tileDir, "cool-product-4.5.6-build.4.pivotal"))
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
				"--release-tarball", cfReleaseTarball,
				"--stemcell-tarball", stemcellTarball,
				"--handcraft", handcraft,
				"--version", "7.8.9-build.4",
				"--final-version", "7.8.9",
				"--product-name", "cool-product-name",
				"--filename-prefix", "cool-product",
				"--output-dir", tileDir)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			archive, err := os.Open(filepath.Join(tileDir, "cool-product-7.8.9-build.4.pivotal"))
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
			Eventually(session.Out).Should(gbytes.Say("Creating empty migrations folder in .pivotal..."))
		})
	})

	Context("when content migrations are provided", func() {
		It("generates the correct content migration file", func() {
			command := exec.Command(pathToMain,
				"--release-tarball", cfReleaseTarball,
				"--stemcell-tarball", stemcellTarball,
				"--handcraft", handcraft,
				"--content-migration", contentMigration,
				"--base-content-migration", baseContentMigration,
				"--version", "7.8.9-build.4",
				"--final-version", "7.8.9",
				"--product-name", "cool-product-name",
				"--filename-prefix", "cool-product",
				"--output-dir", tileDir)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			archive, err := os.Open(filepath.Join(tileDir, "cool-product-7.8.9-build.4.pivotal"))
			Expect(err).NotTo(HaveOccurred())

			archiveInfo, err := archive.Stat()
			Expect(err).NotTo(HaveOccurred())

			zr, err := zip.NewReader(archive, archiveInfo.Size())
			Expect(err).NotTo(HaveOccurred())

			var archivedContentMigration io.ReadCloser
			for _, f := range zr.File {
				if f.Name == "content_migrations/migrations.yml" {
					archivedContentMigration, err = f.Open()
					Expect(err).NotTo(HaveOccurred())
				}
			}

			contents, err := ioutil.ReadAll(archivedContentMigration)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal(`product: my-product
installation_schema_version: "1.6"
to_version: 7.8.9
migrations:
- from_version: 1.6.0-build.315
  rules:
  - selector: product_version
    to: 7.8.9
    type: update
`))
		})
	})

	Context("failure cases", func() {
		Context("when a release tarball does not exist", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain,
					"--release-tarball", "missing-file",
					"--handcraft", "handcraft.yml",
					"--stemcell-tarball", "stemcell.tgz",
					"--version", "6.5.4-build.4",
					"--final-version", "6.5.4",
					"--product-name", "cool-product-name",
					"--filename-prefix", "cool-product",
					"--output-dir", tileDir)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("open missing-file: no such file or directory"))
			})
		})

		Context("when the output directory is not writable", func() {
			It("prints an error and exit 1", func() {
				command := exec.Command(pathToMain,
					"--release-tarball", cfReleaseTarball,
					"--stemcell-tarball", stemcellTarball,
					"--handcraft", handcraft,
					"--version", "5.5.5-build.4",
					"--final-version", "5.5.5",
					"--product-name", "cool-product-name",
					"--filename-prefix", "cool-product",
					"--output-dir", "/path/to/missing/dir")

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(string(session.Err.Contents()))).To(ContainSubstring("no such file or directory"))
			})
		})

		Context("when a content migration does not exist", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain,
					"--release-tarball", cfReleaseTarball,
					"--content-migration", "missing-migration",
					"--base-content-migration", baseContentMigration,
					"--handcraft", handcraft,
					"--stemcell-tarball", stemcellTarball,
					"--version", "6.5.4-build.4",
					"--final-version", "6.5.4",
					"--product-name", "cool-product-name",
					"--filename-prefix", "cool-product",
					"--output-dir", tileDir)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("open missing-migration: no such file or directory"))
			})
		})
	})
})
