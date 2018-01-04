package builder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/builder"
)

var _ = Describe("interpolator", func() {

	const templateYAML = `
name: $( variable "some-variable" )
icon_img: $( icon )
releases:
- $(release "some-release")
stemcell_criteria: $( stemcell )
form_types:
- $( form "some-form" )
job_types:
- $( instance_group "some-instance-group" )
`

	var input builder.InterpolateInput

	BeforeEach(func() {
		input = builder.InterpolateInput{
			Variables: map[string]string{
				"some-variable": "some-value",
			},
			ReleaseManifests: map[string]builder.ReleaseManifest{
				"some-release": builder.ReleaseManifest{
					Name:    "some-release",
					Version: "1.2.3",
					File:    "some-release-1.2.3.tgz",
					SHA1:    "123abc",
				},
			},
			StemcellManifest: builder.StemcellManifest{
				Version:         "2.3.4",
				OperatingSystem: "an-operating-system",
			},
			FormTypes: map[string]interface{}{
				"some-form": builder.Metadata{
					"name":  "some-form",
					"label": "some-form-label",
				},
			},
			IconImage: "some-icon-image",
			InstanceGroups: map[string]interface{}{
				"some-instance-group": builder.Metadata{
					"manifest": "some-manifest",
					"name":     "some-instance-group",
					"provides": "some-link",
					"release":  "some-release",
				},
			},
		}
	})

	It("interpolates metadata templates", func() {
		interpolator := builder.NewInterpolator()
		interpolatedYAML, err := interpolator.Interpolate(input, []byte(templateYAML))
		Expect(err).NotTo(HaveOccurred())
		Expect(interpolatedYAML).To(MatchYAML(`
name: some-value
icon_img: some-icon-image
releases:
- name: some-release
  file: some-release-1.2.3.tgz
  version: 1.2.3
  sha1: 123abc
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
form_types:
- name: some-form
  label: some-form-label
job_types:
- name: some-instance-group
  manifest: some-manifest
  provides: some-link
  release: some-release
`))
		Expect(string(interpolatedYAML)).To(ContainSubstring("file: some-release-1.2.3.tgz\n"))
	})

	It("allows interpolation helpers inside forms", func() {
		input.Variables["some-form-variable"] = "variable-form-label"
		input.FormTypes = map[string]interface{}{
			"some-form": builder.Metadata{
				"name":  "some-form",
				"label": `$( variable "some-form-variable" )`,
			},
		}
		interpolator := builder.NewInterpolator()
		interpolatedYAML, err := interpolator.Interpolate(input, []byte(templateYAML))
		Expect(err).NotTo(HaveOccurred())
		Expect(interpolatedYAML).To(MatchYAML(`
name: some-value
icon_img: some-icon-image
releases:
- name: some-release
  file: some-release-1.2.3.tgz
  version: 1.2.3
  sha1: 123abc
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
form_types:
- name: some-form
  label: variable-form-label
job_types:
- name: some-instance-group
  manifest: some-manifest
  provides: some-link
  release: some-release
`))
	})

	Context("failure cases", func() {
		Context("when the requested form name is not found", func() {
			It("returns an error", func() {
				input.FormTypes = map[string]interface{}{}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find form with key 'some-form'"))
			})
		})

		Context("when the nested form contains invalid templating", func() {
			It("returns an error", func() {
				input.FormTypes = map[string]interface{}{
					"some-form": builder.Metadata{
						"name":  "some-form",
						"label": "$( invalid_helper )",
					},
				}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(templateYAML))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to interpolate value"))
			})
		})

		Context("when template parsing fails", func() {
			It("returns an error", func() {

				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte("$(variable"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("template parsing failed"))
			})
		})

		Context("when template execution fails", func() {
			It("returns an error", func() {

				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(`name: $( variable "some-missing-variable" )`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("template execution failed"))
				Expect(err.Error()).To(ContainSubstring("could not find variable with key"))
			})
		})

		Context("when release tgz file does not exist but is provided", func() {
			It("returns an error", func() {

				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(`releases: [$(release "some-release-does-not-exist")]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find release with name 'some-release-does-not-exist'"))
			})
		})

		Context("when the stemcell helper is used without providing the stemcell", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.StemcellManifest = builder.StemcellManifest{}
				_, err := interpolator.Interpolate(input, []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("stemcell-tarball must be specified"))
			})
		})

		Context("when a specified instance group is not included in the interpolate input", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(`job_types: [$(instance_group "some-instance-group-does-not-exist")]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find instance_group with name 'some-instance-group-does-not-exist'"))
			})
		})

	})

})
