package preprocess_test

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pivotal-cf/kiln/internal/preprocess"
	"gopkg.in/src-d/go-billy.v4/osfs"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		err := preprocess.Run(osfs.New(outputPath), osfs.New(metadataPartsPath), "ert", []string{"ert", "srt"})
		Expect(err).NotTo(HaveOccurred())

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
		err := preprocess.Run(osfs.New(outputPath), osfs.New(metadataPartsPath), "srt", []string{"ert", "srt"})
		Expect(err).NotTo(HaveOccurred())

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

	Context("when tile names contain spaces", func() {
		It("prints an error message", func() {
			err := preprocess.Run(osfs.New(outputPath), osfs.New(metadataPartsPath), "ert", []string{" ert ", "\tsrt"})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when tile name is an strings", func() {
		It("prints an error message", func() {
			err := preprocess.Run(osfs.New(outputPath), osfs.New("test_data/nothing-to-pre-process"), "", []string{})
			Expect(err).To(MatchError(ContainSubstring("empty")))
		})
	})

	Context("when tile name is invalid identifier", func() {
		It("prints an error message", func() {
			err := preprocess.Run(osfs.New(outputPath), osfs.New("test_data/nothing-to-pre-process"), "bad-tile-name", []string{"bad-tile-name"})
			Expect(err).To(MatchError(ContainSubstring("is not a valid identifier")))
		})
	})

	Context("failure cases", func() {
		Context("when the metadata file references a missing key", func() {
			It("errors", func() {
				inputPath := filepath.Join("test_data", "missing-key")
				err := preprocess.Run(osfs.New(outputPath), osfs.New(inputPath), "ert", []string{"ert", "srt"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("some_missing_key")))
			})
		})

		Context("when the metadata file contains a malformed expression", func() {
			It("prints an error message", func() {

				inputPath := filepath.Join("test_data", "malformed-expression")
				err := preprocess.Run(osfs.New(outputPath), osfs.New(inputPath), "ert", []string{"ert", "srt"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("unclosed action")))
			})
		})

		Context("when an unlisted tile name is used", func() {
			It("prints an error message", func() {
				err := preprocess.Run(osfs.New(outputPath), osfs.New(metadataPartsPath), "some-other-tile", []string{"ert", "srt"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(And(ContainSubstring("not provided"), ContainSubstring("some-other-tile"))))
			})
		})
	})
})
