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
some_releases:
- $(release "some-release")
stemcell_criteria: $( stemcell )
some_form_types:
- $( form "some-form" )
some_job_types:
- $( instance_group "some-instance-group" )
version: $( version )
some_property_blueprints:
- $( property "some-templated-property" )
- $( property "some-other-templated-property" )
some_runtime_configs:
- $( runtime_config "some-runtime-config" )
`

	var (
		input        builder.InterpolateInput
		interpolator builder.Interpolator
	)

	BeforeEach(func() {
		interpolator = builder.NewInterpolator()

		input = builder.InterpolateInput{
			Version: "3.4.5",
			Variables: map[string]interface{}{
				"some-variable": "some-value",
			},
			ReleaseManifests: map[string]interface{}{
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
					"name": "some-instance-group",
					"templates": []string{
						"$( job \"some-job\" )",
					},
				},
			},
			Jobs: map[string]interface{}{
				"some-job": builder.Metadata{
					"name":    "some-job",
					"release": "some-release",
				},
			},
			PropertyBlueprints: map[string]interface{}{
				"some-templated-property": builder.Metadata{
					"name":         "some-templated-property",
					"type":         "boolean",
					"configurable": true,
					"default":      false,
				},
				"some-other-templated-property": builder.Metadata{
					"name":         "some-other-templated-property",
					"type":         "string",
					"configurable": false,
					"default":      "some-value",
				},
			},
			RuntimeConfigs: map[string]interface{}{
				"some-runtime-config": builder.Metadata{
					"name":           "some-runtime-config",
					"runtime_config": "some-addon-runtime-config",
				},
			},
		}
	})

	It("interpolates metadata templates", func() {
		interpolatedYAML, err := interpolator.Interpolate(input, []byte(templateYAML))
		Expect(err).NotTo(HaveOccurred())
		Expect(interpolatedYAML).To(MatchYAML(`
name: some-value
icon_img: some-icon-image
some_releases:
- name: some-release
  file: some-release-1.2.3.tgz
  version: 1.2.3
  sha1: 123abc
stemcell_criteria:
  version: 2.3.4
  os: an-operating-system
some_form_types:
- name: some-form
  label: some-form-label
some_job_types:
- name: some-instance-group
  templates:
  - name: some-job
    release: some-release
version: 3.4.5
some_property_blueprints:
- name: some-templated-property
  type: boolean
  configurable: true
  default: false
- name: some-other-templated-property
  type: string
  configurable: false
  default: some-value
some_runtime_configs:
- name: some-runtime-config
  runtime_config: |
    some-addon-runtime-config
`))
		Expect(string(interpolatedYAML)).To(ContainSubstring("file: some-release-1.2.3.tgz\n"))
	})

	It("allows interpolation helpers inside forms", func() {
		templateYAML := `
---
some_form_types:
- $( form "some-form" )`

		input := builder.InterpolateInput{
			Variables: map[string]interface{}{
				"some-form-variable": "variable-form-label",
			},
			FormTypes: map[string]interface{}{
				"some-form": builder.Metadata{
					"name":  "some-form",
					"label": `$( variable "some-form-variable" )`,
				},
			},
		}

		interpolatedYAML, err := interpolator.Interpolate(input, []byte(templateYAML))
		Expect(err).NotTo(HaveOccurred())
		Expect(interpolatedYAML).To(MatchYAML(`
some_form_types:
- name: some-form
  label: variable-form-label
`))
	})

	Context("when the runtime config is provided", func() {

		var templateYAML string
		var input builder.InterpolateInput

		BeforeEach(func() {
			templateYAML = `
---
some_runtime_configs:
- $( runtime_config "some-runtime-config" )`

			input = builder.InterpolateInput{
				ReleaseManifests: map[string]interface{}{
					"some-release": builder.ReleaseManifest{
						Name:    "some-release",
						Version: "1.2.3",
						File:    "some-release-1.2.3.tgz",
						SHA1:    "123abc",
					},
				},
				RuntimeConfigs: map[string]interface{}{
					"some-runtime-config": builder.Metadata{
						"name": "some-runtime-config",
						"runtime_config": `releases:
- $( release "some-release" )`,
					},
				},
			}
		})

		It("allows interpolation helpers inside runtime_configs", func() {
			interpolatedYAML, err := interpolator.Interpolate(input, []byte(templateYAML))
			Expect(err).NotTo(HaveOccurred())
			Expect(interpolatedYAML).To(MatchYAML(`
some_runtime_configs:
- name: some-runtime-config
  runtime_config: |
    releases:
    - file: some-release-1.2.3.tgz
      name: some-release
      sha1: 123abc
      version: 1.2.3
`))
		})

		Context("when the interpolated runtime config does not have a runtime_config key", func() {
			JustBeforeEach(func() {
				input.RuntimeConfigs = map[string]interface{}{
					"some-runtime-config": builder.Metadata{
						"name": "some-runtime-config",
					},
				}
			})
			It("does not error", func() {
				interpolatedYAML, err := interpolator.Interpolate(input, []byte(templateYAML))
				Expect(err).NotTo(HaveOccurred())
				Expect(interpolatedYAML).To(MatchYAML(`
some_runtime_configs:
- name: some-runtime-config
`))
			})
		})

		Context("when the interpolated runtime config contains invalid YAML", func() {
			JustBeforeEach(func() {
				input.RuntimeConfigs = map[string]interface{}{
					"some-runtime-config": builder.Metadata{
						"name":           "some-runtime-config",
						"runtime_config": "\tinvalid yaml",
					},
				}
			})
			It("errors", func() {
				_, err := interpolator.Interpolate(input, []byte(templateYAML))
				Expect(err).To(HaveOccurred())
			})
		})

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

		Context("when the requested property blueprint is not found", func() {
			It("returns an error", func() {
				input.PropertyBlueprints = map[string]interface{}{}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find property blueprint with name 'some-templated-property'"))
			})
		})

		Context("when the requested runtime config is not found", func() {
			It("returns an error", func() {
				input.RuntimeConfigs = map[string]interface{}{}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find runtime_config with name 'some-runtime-config'"))
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

		Context("when the version helper is used without providing the version flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.Version = ""
				_, err := interpolator.Interpolate(input, []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("version flag must be specified"))
			})
		})

		Context("when the stemcell helper is used without providing the stemcell", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.StemcellManifest = nil
				_, err := interpolator.Interpolate(input, []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("stemcell-tarball flag must be specified"))
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
		Context("when a specified job is not included in the interpolate input", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, []byte(`job: [$(job "some-job-does-not-exist")]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find job with name 'some-job-does-not-exist'"))
			})
		})

	})

})
