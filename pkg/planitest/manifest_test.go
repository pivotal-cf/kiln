package planitest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/planitest"
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
			m := planitest.Manifest(yamlContent)
			job, err := m.FindInstanceGroupJob("some-instance-group", "some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(job).To(MatchYAML("name: some-job"))
		})

		When("the input is not valid yaml", func() {
			It("errors", func() {
				notYAML := `{{{`
				m := planitest.Manifest(notYAML)
				_, err := m.FindInstanceGroupJob("an", "error")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse manifest"))
			})
		})

		When("the instance group does not exist", func() {
			It("errors", func() {
				m := planitest.Manifest(yamlContent)
				_, err := m.FindInstanceGroupJob("doesnot", "exist")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("doesnot.*exist"))
			})
		})
	})

	Describe("Property", func() {
		It("returns the matching property value", func() {
			m := planitest.Manifest(yamlContent)
			job, err := m.FindInstanceGroupJob("some-instance-group-with-property", "some-job")
			Expect(err).NotTo(HaveOccurred())

			value, err := job.Property("nested/property/structure")
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(BeTrue())
		})

		When("the property does not exist", func() {
			It("errors", func() {
				m := planitest.Manifest(yamlContent)
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
			m := planitest.Manifest(yamlContent)
			prop, err := m.Path("/instance_groups/name=some-instance-group-with-property/jobs/name=some-job/properties/nested/property/structure")
			Expect(err).NotTo(HaveOccurred())
			Expect(prop).To(BeTrue())
		})

		When("the property does not exist", func() {
			It("errors", func() {
				m := planitest.Manifest(yamlContent)
				_, err := m.Path("/doesnot/exist")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("doesnot.*exist"))
			})
		})
	})

	Describe("String", func() {
		It("returns YAML as the string representation", func() {
			m := planitest.Manifest("name: some-deployment")
			Expect(m).To(MatchYAML("name: some-deployment"))
		})
	})
})
