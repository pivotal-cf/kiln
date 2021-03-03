package main_test

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("preprocess", func() {
	var (
		outputPath        string
		metadataPartsPath string
	)

	BeforeEach(func() {
		var err error
		outputPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		metadataPartsPath = filepath.Join("test_data", "valid")
	})

	It("processes the templates files for the ERT", func() {
		command := exec.Command(pathToMain,
			"--tile-name", "ert",
			"--input-path", metadataPartsPath,
			"--output-path", outputPath,
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0))

		baseFilePath := filepath.Join(outputPath, "base.yml")
		contents, err := ioutil.ReadFile(baseFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(MatchYAML(`---
metadata_version: some-metadata-version
name: ert
provides_product_versions:
- name: ert-product
requires_product_versions:
- name: some-other-product
  version: 1.2.3.4
product_version: some-product-version
minimum_version_for_upgrade: some-minimum-version
label: some-label
description: some-description
icon_image: some-icon
rank: 90
serial: false
job_types:
- $( instance_group "some_instance_group" )
post_deploy_errands:
  - name: some-errand
variables:
- name: root-ca
  type: rsa
  options:
    is_ca: true
`))

		instanceGroupPath := filepath.Join(outputPath, "instance_groups", "some_instance_group.yml")
		contents, err = ioutil.ReadFile(instanceGroupPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(MatchYAML(`---
name: some_instance_group
label: Some Instance Group
templates:
- $( job "some_job" )
- $( job "some_other_job" )
`))
	})

	It("processes the templates files for the SRT", func() {
		command := exec.Command(pathToMain,
			"--tile-name", "srt",
			"--input-path", metadataPartsPath,
			"--output-path", outputPath,
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0))

		baseFilePath := filepath.Join(outputPath, "base.yml")
		contents, err := ioutil.ReadFile(baseFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(MatchYAML(`---
metadata_version: some-metadata-version
name: srt
provides_product_versions:
- name: srt-product
requires_product_versions:
- name: some-other-product
  version: 1.2.3.4
product_version: some-product-version
minimum_version_for_upgrade: some-minimum-version
label: some-label
description: some-description
icon_image: some-icon
rank: 90
serial: false
job_types:
- $( instance_group "some_instance_group" )
post_deploy_errands:
  - name: some-errand
variables:
- name: root-ca
  type: rsa
  options:
    is_ca: true
`))

		instanceGroupPath := filepath.Join(outputPath, "instance_groups", "some_instance_group.yml")
		contents, err = ioutil.ReadFile(instanceGroupPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(MatchYAML(`---
name: some_instance_group
label: Some Instance Group
templates:
- $( job "placeholder" )
`))
	})

	Context("failure cases", func() {
		Context("when the metadata file references a missing key", func() {
			It("errors", func() {
				command := exec.Command(pathToMain,
					"--tile-name", "ert",
					"--input-path", filepath.Join("test_data", "missing-key"),
					"--output-path", outputPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(string(session.Err.Contents())).To(ContainSubstring("some_missing_key"))
			})
		})

		Context("when the metadata file contains a malformed expression", func() {
			It("prints an error message", func() {
				command := exec.Command(pathToMain,
					"--tile-name", "ert",
					"--input-path", filepath.Join("test_data", "malformed-expression"),
					"--output-path", outputPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("unclosed action"))
			})
		})

		Context("when the --tile-name flag is not provided", func() {
			It("prints an error message", func() {
				command := exec.Command(pathToMain,
					"--input-path", metadataPartsPath,
					"--output-path", outputPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("please provide a tile name using the --tile-name option"))
			})
		})

		Context("when the --input-path flag is not provided", func() {
			It("prints an error message", func() {
				command := exec.Command(pathToMain,
					"--tile-name", "ert",
					"--output-path", outputPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("please provide a metadata parts directory path using the --input-path option"))
			})
		})

		Context("when the --output-path flag is not provided", func() {
			It("prints an error message", func() {
				command := exec.Command(pathToMain,
					"--tile-name", "ert",
					"--input-path", metadataPartsPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("please provide an output directory path using the --output-path option"))
			})
		})

		Context("when the output directory path is actually a file", func() {
			var existingFilePath string

			BeforeEach(func() {
				existingFile, err := ioutil.TempFile("", "")
				Expect(err).NotTo(HaveOccurred())

				existingFilePath = existingFile.Name()
			})

			It("prints an error message", func() {
				command := exec.Command(pathToMain,
					"--tile-name", "ert",
					"--input-path", metadataPartsPath,
					"--output-path", existingFilePath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("not a directory"))
			})
		})

		Context("when an unsupported tile name is specified", func() {
			It("prints an error message", func() {
				command := exec.Command(pathToMain,
					"--tile-name", "some-other-tile",
					"--input-path", metadataPartsPath,
					"--output-path", outputPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("unsupported tile name: some-other-tile"))
			})
		})
	})
})
