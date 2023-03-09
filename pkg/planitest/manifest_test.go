package planitest_test

import (
	. "github.com/pivotal-cf/kiln/pkg/planitest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	const yamlContent = `---
name: some-deployment
instance_groups:
- name: some-instance-group
  jobs:
  - name: some-job
- name: some-instance-group-with-property
  jobs:
  - name: some-job
    properties:
      non-yaml: "{mem_relative,1.0}"
      nested:
        property:
          structure: true`

	Describe("FindInstanceGroupJob", func() {
		It("returns the matching job", func() {
			m := Manifest(yamlContent)
			job, err := m.FindInstanceGroupJob("some-instance-group", "some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(job).To(MatchYAML("name: some-job"))
		})

		When("the input is not valid yaml", func() {
			It("errors", func() {
				notYAML := `{{{`
				m := Manifest(notYAML)
				_, err := m.FindInstanceGroupJob("an", "error")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse manifest"))
			})
		})

		When("the instance group does not exist", func() {
			It("errors", func() {
				m := Manifest(yamlContent)
				_, err := m.FindInstanceGroupJob("doesnot", "exist")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("doesnot.*exist"))
			})
		})
	})

	Describe("Property", func() {
		It("returns the matching property value", func() {
			m := Manifest(yamlContent)
			job, err := m.FindInstanceGroupJob("some-instance-group-with-property", "some-job")
			Expect(err).NotTo(HaveOccurred())

			value, err := job.Property("nested/property/structure")
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(BeTrue())
		})

		When("the property does not exist", func() {
			It("errors", func() {
				m := Manifest(yamlContent)
				job, err := m.FindInstanceGroupJob("some-instance-group-with-property", "some-job")
				Expect(err).NotTo(HaveOccurred())

				_, err = job.Property("doesnot/exist")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("doesnot.*exist"))
			})
		})
	})

	Describe("Path", func() {
		It("returns the matching value", func() {
			m := Manifest(yamlContent)
			prop, err := m.Path("/instance_groups/name=some-instance-group-with-property/jobs/name=some-job/properties/nested/property/structure")
			Expect(err).NotTo(HaveOccurred())
			Expect(prop).To(BeTrue())
		})

		When("the property does not exist", func() {
			It("errors", func() {
				m := Manifest(yamlContent)
				_, err := m.Path("/doesnot/exist")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("doesnot.*exist"))
			})
		})
	})

	Describe("String", func() {
		It("returns YAML as the string representation", func() {
			m := Manifest("name: some-deployment")
			Expect(m).To(MatchYAML("name: some-deployment"))
		})
	})
})
