package builder_test

import (
	"bytes"
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/builder"
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
some_bosh_variables:
- $( bosh_variable "some-bosh-variable" )
some_regex_replace:
- https://some-link/$( regexReplaceAll "^\"?([0-9]+)\\.([0-9]+)\\.([0-9]+).*$" version "${1}-${2}-${3}" )/index.html

selected_value: $( release "some-release" | select "version" )
`

	var (
		input        builder.InterpolateInput
		interpolator builder.Interpolator
	)

	BeforeEach(func() {
		interpolator = builder.NewInterpolator()

		input = builder.InterpolateInput{
			Version: "3.4.5",
			BOSHVariables: map[string]interface{}{
				"some-bosh-variable": builder.Metadata{
					"name": "some-bosh-variable",
					"type": "some-bosh-type",
				},
			},
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
					"runtime_config": "some-addon-runtime-config\n",
				},
			},
		}
	})

	It("interpolates yaml correctly", func() {

		type namedThing struct {
			Name string `yaml:"name"`
		}

		var t namedThing

		badYamlWithName := `templates:
  $(if ert)
  - foo
  $(end)
name: foo
`

		err := yaml.Unmarshal([]byte(badYamlWithName), &t)
		Expect(err).NotTo(HaveOccurred())

		Expect(t.Name).To(Equal("foo"))
	})

	It("interpolates metadata templates", func() {
		interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(templateYAML))
		Expect(err).NotTo(HaveOccurred())
		Expect(interpolatedYAML).To(HelpfullyMatchYAML(`
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
some_bosh_variables:
- name: some-bosh-variable
  type: some-bosh-type
some_regex_replace:
- https://some-link/3-4-5/index.html

selected_value: 1.2.3	
`))
		Expect(string(interpolatedYAML)).To(ContainSubstring("file: some-release-1.2.3.tgz\n"))
	})

	It("allows interpolation helpers inside forms", func() {
		templateYAML := `
---
some_form_types:
- $( form "some-form" )`

		input = builder.InterpolateInput{
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

		interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(templateYAML))
		Expect(err).NotTo(HaveOccurred())
		Expect(interpolatedYAML).To(HelpfullyMatchYAML(`
some_form_types:
- name: some-form
  label: variable-form-label
`))
	})

	Context("when multiple stemcells are specified", func() {
		var templateYAML string

		BeforeEach(func() {
			input = builder.InterpolateInput{
				ReleaseManifests: map[string]interface{}{
					"some-release": builder.ReleaseManifest{
						Name:    "some-release",
						Version: "1.2.3",
						File:    "some-release-1.2.3.tgz",
						SHA1:    "123abc",
					},
				},
				StemcellManifests: map[string]interface{}{
					"windows": builder.StemcellManifest{
						OperatingSystem: "windows",
						Version:         "2019.4",
					},
					"centOS": builder.StemcellManifest{
						OperatingSystem: "centOS",
						Version:         "5.4",
					},
				},
			}
		})

		It("interpolates stemcell keys properly", func() {
			templateYAML = `
---
stemcell_criteria: $( stemcell "centOS" )
additional_stemcells_criteria:
- $( stemcell "windows")
`

			interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(templateYAML))
			Expect(err).NotTo(HaveOccurred())
			Expect(interpolatedYAML).To(HelpfullyMatchYAML(`
---
stemcell_criteria:
  os: centOS
  version: "5.4"
additional_stemcells_criteria:
- os: windows
  version: "2019.4"`,
			))
		})

		It("returns error because stemcell helper needs an argument", func() {
			templateYAML = `
---
stemcell_criteria: $( stemcell )
additional_stemcells_criteria:
- $( stemcell )
`

			_, err := interpolator.Interpolate(input, "", []byte(templateYAML))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("stemcell template helper requires osname argument if multiple stemcells are specified"))
		})
	})

	Context("when only one stemcell is specified", func() {
		var templateYAML string

		BeforeEach(func() {
			templateYAML = `
---
stemcell_criteria: $( stemcell )`

			input = builder.InterpolateInput{
				ReleaseManifests: map[string]interface{}{
					"some-release": builder.ReleaseManifest{
						Name:    "some-release",
						Version: "1.2.3",
						File:    "some-release-1.2.3.tgz",
						SHA1:    "123abc",
					},
				},
				StemcellManifests: map[string]interface{}{
					"centOS": builder.StemcellManifest{
						OperatingSystem: "centOS",
						Version:         "5.4",
					},
				},
			}
		})

		It("interpolates stemcell key properly", func() {
			interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(templateYAML))
			Expect(err).NotTo(HaveOccurred())
			Expect(interpolatedYAML).To(HelpfullyMatchYAML(`
---
stemcell_criteria:
  os: centOS
  version: "5.4"
`,
			))
		})

	})

	Context("when the runtime config is provided", func() {
		var templateYAML string

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
			interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(templateYAML))
			Expect(err).NotTo(HaveOccurred())

			var output map[string]interface{}
			err = yaml.Unmarshal(interpolatedYAML, &output)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(HaveKey("some_runtime_configs"))
			configs, ok := output["some_runtime_configs"].([]interface{})
			Expect(ok).To(BeTrue())
			config, ok := configs[0].(map[interface{}]interface{})
			Expect(ok).To(BeTrue())

			Expect(config).To(HaveKeyWithValue("name", "some-runtime-config"))
			Expect(config["runtime_config"]).To(HelpfullyMatchYAML(`
releases:
- file: some-release-1.2.3.tgz
  name: some-release
  sha1: 123abc
  version: 1.2.3`))
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
				interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(templateYAML))
				Expect(err).NotTo(HaveOccurred())
				Expect(interpolatedYAML).To(HelpfullyMatchYAML(`
some_runtime_configs:
- name: some-runtime-config
`))
			})
		})
	})

	Context("when release tgz file does not exist and stub releases is true", func() {
		It("creates stub values for file, sha1, and version", func() {

			interpolator := builder.NewInterpolator()
			input.StubReleases = true
			interpolatedYAML, err := interpolator.Interpolate(input, "", []byte(`releases: [$(release "stub-release")]`))

			Expect(err).NotTo(HaveOccurred())
			Expect(interpolatedYAML).To(HelpfullyMatchYAML(`releases:
- name: stub-release
  file: stub-release-UNKNOWN.tgz
  sha1: dead8e1ea5e00dead8e1ea5ed00ead8e1ea5e000
  version: UNKNOWN`))
		})
	})

	Context("failure cases", func() {
		Context("when the requested form name is not found", func() {
			It("returns an error", func() {
				input.FormTypes = map[string]interface{}{}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find form with key 'some-form'"))
			})
		})

		Context("when the requested property blueprint is not found", func() {
			It("returns an error", func() {
				input.PropertyBlueprints = map[string]interface{}{}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find property blueprint with name 'some-templated-property'"))
			})
		})

		Context("when the requested runtime config is not found", func() {
			It("returns an error", func() {
				input.RuntimeConfigs = map[string]interface{}{}
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

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
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to interpolate value"))
			})
		})

		Context("when template parsing fails", func() {
			It("returns an error", func() {

				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte("$(variable"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed when parsing a template"))
			})
		})

		Context("when template execution fails", func() {
			It("returns an error", func() {

				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`name: $( variable "some-missing-variable" )`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed when rendering a template"))
				Expect(err.Error()).To(ContainSubstring("could not find variable with key"))
			})
		})

		Context("when release tgz file does not exist but is provided", func() {
			It("returns an error", func() {

				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`releases: [$(release "some-release-does-not-exist")]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find release with name 'some-release-does-not-exist'"))
			})
		})

		Context("when the bosh_variable helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.BOSHVariables = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--bosh-variables-directory must be specified"))
			})
		})

		Context("when the form helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.FormTypes = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--forms-directory must be specified"))
			})
		})

		Context("when the property helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.PropertyBlueprints = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--properties-directory must be specified"))
			})
		})

		Context("when the release helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.ReleaseManifests = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing ReleaseManifests"))
			})
		})

		Context("when the stemcell helper is used without any stemcells", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.StemcellManifest = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("stemcell specification must be provided through either --stemcells-directory or --kilnfile"))
			})
		})

		Context("when the stemcell named helper is used and no stemcell directories", func() {
			It("returns an error", func() {
				const templateWithStemcellName = `
some_releases:
- $(release "some-release")
stemcell_criteria: $( stemcell "windows" )
`
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(templateWithStemcellName))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("$( stemcell \"<osname>\" ) cannot be used without --stemcells-directory being provided"))
			})
		})

		Context("when the version helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.Version = ""
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--version must be specified"))
			})
		})

		Context("when the variable helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.Variables = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--variable or --variables-file must be specified"))
			})
		})

		Context("when the icon helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.IconImage = ""
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--icon must be specified"))
			})
		})

		Context("when the instance_group helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.InstanceGroups = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--instance-groups-directory must be specified"))
			})
		})

		Context("when the job helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.Jobs = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--jobs-directory must be specified"))
			})
		})

		Context("when the runtime_config helper is used without providing the flag", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				input.RuntimeConfigs = nil
				_, err := interpolator.Interpolate(input, "", []byte(templateYAML))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("--runtime-configs-directory must be specified"))
			})
		})

		Context("when a specified instance group is not included in the interpolate input", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`job_types: [$(instance_group "some-instance-group-does-not-exist")]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find instance_group with name 'some-instance-group-does-not-exist'"))
			})
		})

		Context("when a specified job is not included in the interpolate input", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`job: [$(job "some-job-does-not-exist")]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find job with name 'some-job-does-not-exist'"))
			})
		})

		Context("input to the select function cannot be JSON unmarshalled", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`job: [$( "%%%" | select "value" )]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not JSON unmarshal \"%%%\": invalid character"))
			})
		})

		Context("input to the select function cannot be JSON unmarshalled", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`release: [$( release "some-release" | select "key-not-there" )]`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not select \"key-not-there\", key does not exist"))
			})
		})

		Context("input to regexReplaceAll is not valid regex", func() {
			It("returns an error", func() {
				interpolator := builder.NewInterpolator()
				_, err := interpolator.Interpolate(input, "", []byte(`regex: $( regexReplaceAll "**" "" "" )`))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("regex"))
			})
		})
	})
})

func TestPreProcess(t *testing.T) {
	t.Run("when the metadata file contains a malformed expression", func(t *testing.T) {
		please := NewWithT(t)

		partYML := `---
metadata_version: some-metadata-version
provides_product_versions:
- name: Malformed template expression {{ tile |`

		err := builder.PreProcessMetadataWithTileFunction(nil, "m.yml", io.Discard, []byte(partYML))
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("unclosed action")),
		))
	})

	t.Run("when the metadata file references a missing key", func(t *testing.T) {
		please := NewWithT(t)

		partYML := `---
metadata_version: some-metadata-version
name: {{.some_missing_key}}
`
		err := builder.PreProcessMetadataWithTileFunction(nil, "m.yml", io.Discard, []byte(partYML))
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("some_missing_key")),
		))
	})

	t.Run("when tile-name is set", func(t *testing.T) {
		please := NewWithT(t)

		partYML := `---
tile: {{if eq tile "ert" -}}
	big-foot
{{- else if eq tile "srt" -}}
	small-foot
{{- end -}}
`
		var buf bytes.Buffer
		err := builder.PreProcessMetadataWithTileFunction(map[string]interface{}{"tile-name": "ERT"}, "m.yml", &buf, []byte(partYML))
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(buf.Bytes()).To(MatchYAML(`tile: big-foot`))

		buf.Reset()

		err = builder.PreProcessMetadataWithTileFunction(map[string]interface{}{"tile-name": "SRT"}, "m.yml", &buf, []byte(partYML))
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(buf.Bytes()).To(MatchYAML(`tile: small-foot`))
	})

	t.Run("when tile-name value has the wrong type", func(t *testing.T) {
		please := NewWithT(t)

		partYML := `tile: {{tile}}`
		err := builder.PreProcessMetadataWithTileFunction(map[string]interface{}{"tile-name": 27}, "m.yml", io.Discard, []byte(partYML))
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("expected string")),
		))
	})

	t.Run("when tile-name does not exist", func(t *testing.T) {
		please := NewWithT(t)

		partYML := `tile: {{tile}}`
		err := builder.PreProcessMetadataWithTileFunction(make(map[string]interface{}), "m.yml", io.Discard, []byte(partYML))
		please.Expect(err).To(And(
			HaveOccurred(),
			MatchError(ContainSubstring("could not find variable")),
		))
	})
}
