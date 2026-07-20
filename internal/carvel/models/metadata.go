package models

import "github.com/pivotal-cf/kiln/pkg/proofing"

type Metadata struct {
	Name                              string               `yaml:"name"`
	ProductVersion                    string               `yaml:"product_version"`
	IconImage                         string               `yaml:"icon_image"`
	Label                             string               `yaml:"label"`
	MetadataVersion                   string               `yaml:"metadata_version"`
	MinimumVersionForUpgrade          string               `yaml:"minimum_version_for_upgrade"`
	Rank                              int                  `yaml:"rank"`
	Serial                            bool                 `yaml:"serial"`
	PropertyBlueprints                []string             `yaml:"property_blueprints"`
	FormTypes                         []string             `yaml:"form_types"`
	Variables                         []proofing.Variable  `yaml:"variables"`
	PackageInstalls                   []string             `yaml:"package_installs"`
	CompatibleKubernetesDistributions []ProductVersion     `yaml:"compatible_kubernetes_distributions,omitempty"`
	// AdditionalReleases declares locally-built BOSH releases that kiln should
	// splice into the runtime-config addon alongside the auto-generated tile
	// release. Each entry's TarballPath is resolved relative to the tile source
	// directory; kiln copies it verbatim into .carvel-tile/releases/ and adds
	// the release + declared jobs to the runtime-config addon's releases:/jobs:
	// lists. The tile's own build.sh is responsible for producing the tarball
	// before invoking kiln carvel bake.
	AdditionalReleases []AdditionalRelease `yaml:"additional_releases,omitempty"`
	// PreInstallHooks/PostInstallHooks declare adapter jobs kiln should
	// synthesize directly into the tile's own auto-generated release, each
	// depositing a single delegating script at the conventional hook path
	// consumed by a provider tile's generic hook-runner (see
	// tanzu-vks-provider/docs/2026-07-16-generic-consumer-hook-contract.md).
	// Job names are auto-prefixed with the tile's own name to avoid collisions
	// with other consumer tiles' hook jobs on the same shared VM.
	PreInstallHooks  []HookDeclaration `yaml:"pre_install_hooks,omitempty"`
	PostInstallHooks []HookDeclaration `yaml:"post_install_hooks,omitempty"`
}

// HookDeclaration declares one hook adapter job for kiln to synthesize.
type HookDeclaration struct {
	// Name becomes the synthesized job name, auto-prefixed with the tile's
	// own name unless already prefixed — e.g. "smoke-tests-post-install-hook"
	// on tile "ear-k8s-runtime" becomes job
	// "ear-k8s-runtime-smoke-tests-post-install-hook".
	Name string `yaml:"name"`
	// Command is the single command the synthesized script execs —
	// typically another already-colocated job's own entrypoint, e.g.
	// /var/vcap/jobs/smoke_tests/bin/run.
	Command string `yaml:"command"`
}

// AdditionalRelease declares a locally-built BOSH release to include alongside
// the auto-generated tile release in the carvel runtime-config addon.
type AdditionalRelease struct {
	// Name is the BOSH release name (must match the name used in bosh create-release).
	Name string `yaml:"name"`
	// Version is the BOSH release version (e.g. "dev" for POC builds).
	Version string `yaml:"version"`
	// TarballPath is the path to the pre-built release tarball, relative to
	// the tile source directory.
	TarballPath string `yaml:"tarball_path"`
	// Jobs lists jobs from this release to add to the addon's jobs: list.
	// Each entry can carry an optional properties block that is marshaled
	// verbatim into the generated runtime-config.
	Jobs []AdditionalJob `yaml:"jobs"`
}

// AdditionalJob names a job within an AdditionalRelease and carries an
// optional properties block. It is the input shape (parsed from base.yml);
// models.Job is the output shape (written into the generated runtime-config).
type AdditionalJob struct {
	Name       string                 `yaml:"name"`
	Properties map[string]interface{} `yaml:"properties,omitempty"`
}
