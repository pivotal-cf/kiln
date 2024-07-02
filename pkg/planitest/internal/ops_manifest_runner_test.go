package internal_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/planitest/internal"
	"github.com/pivotal-cf/kiln/pkg/planitest/internal/fakes"
)

var _ = Describe("OMRunner", func() {
	var (
		cmdRunner         *fakes.CommandRunner
		opsManifestRunner internal.OpsManifestRunner
	)

	BeforeEach(func() {
		cmdRunner = &fakes.CommandRunner{}
		opsManifestRunner = internal.NewOpsManifestRunner(cmdRunner, internal.RealIO)
	})

	Describe("GetManifest", func() {
		BeforeEach(func() {
			cmdRunner.RunReturns(`{
	"name": "cf-some-guid",
	"releases": [
		{
			"name": "some-release",
			"version": "1.2.3"
		}
	]
}`, "", nil)
		})

		It("shells out to ops-manifest to get the manifest", func() {
			config := `{
				".properties.some-properties": "some-value"
}`
			manifest, err := opsManifestRunner.GetManifest(config, "some/metadata/path")
			Expect(err).ToNot(HaveOccurred())

			Expect(cmdRunner.RunCallCount()).To(Equal(1))
			command, args := cmdRunner.RunArgsForCall(0)
			Expect(command).To(Equal("ops-manifest"))
			Expect(args).To(HaveLen(4))
			Expect(args[0]).To(ContainSubstring("--config-file"))
			Expect(args[1]).To(HaveSuffix(".yml"))
			Expect(args[2]).To(ContainSubstring("--metadata-path"))
			Expect(args[3]).To(ContainSubstring("some/metadata/path"))

			Expect(manifest).To(Equal(map[string]any{
				"name": "cf-some-guid",
				"releases": []any{
					map[any]any{
						"name":    "some-release",
						"version": "1.2.3",
					},
				},
			}))
		})

		When("the runner was instantiated with additional arguments", func() {
			const (
				tasMetadataPath     = "/path/to/tas/metadata.yml"
				tasConfigPath       = "/path/to/tas/config.yml"
				dollarOverridesPath = "/path/to/dollar/overrides.yml"
			)

			BeforeEach(func() {
				opsManifestRunner = internal.NewOpsManifestRunner(cmdRunner, internal.RealIO,
					"--tas-metadata-path", tasMetadataPath,
					"--tas-config-file", tasConfigPath,
					"--dollar-accessor-values-file", dollarOverridesPath,
				)
			})

			It("passes those flags when it shells out to ops-manifest to get the manifest", func() {
				config := `{
				".properties.some-properties": "some-value"
}`
				manifest, err := opsManifestRunner.GetManifest(config, "some/metadata/path")
				Expect(err).ToNot(HaveOccurred())

				Expect(cmdRunner.RunCallCount()).To(Equal(1))
				command, args := cmdRunner.RunArgsForCall(0)
				Expect(command).To(Equal("ops-manifest"))
				Expect(args).To(HaveLen(10))
				Expect(args[0]).To(ContainSubstring("--config-file"))
				Expect(args[1]).To(HaveSuffix(".yml"))
				Expect(args[2]).To(ContainSubstring("--metadata-path"))
				Expect(args[3]).To(ContainSubstring("some/metadata/path"))
				Expect(args[4]).To(Equal("--tas-metadata-path"))
				Expect(args[5]).To(Equal(tasMetadataPath))
				Expect(args[6]).To(Equal("--tas-config-file"))
				Expect(args[7]).To(Equal(tasConfigPath))
				Expect(args[8]).To(Equal("--dollar-accessor-values-file"))
				Expect(args[9]).To(Equal(dollarOverridesPath))

				Expect(manifest).To(Equal(map[string]any{
					"name": "cf-some-guid",
					"releases": []any{
						map[any]any{
							"name":    "some-release",
							"version": "1.2.3",
						},
					},
				}))
			})
		})

		Context("failure cases", func() {
			When("the manifest cannot be retrieved", func() {
				It("errors", func() {
					cmdRunner.RunReturns("", "stderr output", errors.New("some error"))

					_, err := opsManifestRunner.GetManifest("", "")
					Expect(err).To(MatchError(`Unable to retrieve manifest: some error: stderr output`))
				})
			})

			When("the manifest response is not well-formed YAML", func() {
				It("errors", func() {
					cmdRunner.RunReturns("not-well-formed-yaml", "", nil)

					_, err := opsManifestRunner.GetManifest("", "")
					Expect(err).To(MatchError(HavePrefix("Unable to unmarshal yaml")))
				})
			})
		})
	})
})
