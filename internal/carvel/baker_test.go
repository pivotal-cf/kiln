package carvel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/proofing"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Carvel Baker", func() {
	Context("generateManifestTemplate", func() {
		var template string

		BeforeEach(func() {
			template = generateManifestTemplate("test-install", "")
		})

		It("generates a ServiceAccount", func() {
			Expect(template).To(ContainSubstring("kind: ServiceAccount"))
			Expect(template).To(ContainSubstring(`name: <%= p("test-install.name") %>-sa`))
		})

		It("generates a ClusterRole (not a namespaced Role)", func() {
			Expect(template).To(ContainSubstring("kind: ClusterRole"))
			Expect(template).NotTo(ContainSubstring("kind: Role\n"))
			Expect(template).To(ContainSubstring(`name: <%= p("test-install.name") %>-sa-cluster-role`))
		})

		It("generates a ClusterRoleBinding (not a namespaced RoleBinding)", func() {
			Expect(template).To(ContainSubstring("kind: ClusterRoleBinding"))
			Expect(template).NotTo(ContainSubstring("kind: RoleBinding\n"))
			Expect(template).To(ContainSubstring(`name: <%= p("test-install.name") %>-sa-cluster-role-binding`))
		})

		It("generates a Secret for values", func() {
			Expect(template).To(ContainSubstring("kind: Secret"))
			Expect(template).To(ContainSubstring(`name: <%= p("test-install.name") %>-values`))
			Expect(template).To(ContainSubstring("stringData:"))
			Expect(template).To(ContainSubstring("values.yaml: |"))
		})

		It("generates a PackageInstall resource", func() {
			Expect(template).To(ContainSubstring("kind: PackageInstall"))
			Expect(template).To(ContainSubstring("apiVersion: packaging.carvel.dev/v1alpha1"))
			Expect(template).To(ContainSubstring(`name: <%= p("test-install.name") %>`))
			Expect(template).To(ContainSubstring(`serviceAccountName: <%= p("test-install.name") %>-sa`))
		})

		It("uses BOSH link for content-namespace with fallback to default", func() {
			Expect(template).To(ContainSubstring(`<%= link("cluster").p("content-namespace") rescue "default" %>`))
		})

		It("injects content-namespace from BOSH link into values context", func() {
			Expect(template).To(ContainSubstring(`values["context"]["namespace"] = link("cluster").p("content-namespace") rescue "default"`))
		})

		It("handles YAML conversion for string values", func() {
			Expect(template).To(ContainSubstring(`values = YAML.load(values) if values.is_a?(String)`))
		})

		Context("with overlay content", func() {
			It("includes overlay content before YAML.dump", func() {
				overlay := `<% values["syslog_agent"]["cache"]["url"] = "https://1.2.3.4:9000" %>`
				tmpl := generateManifestTemplate("test-install", overlay)
				Expect(tmpl).To(ContainSubstring(overlay))
				overlayIdx := strings.Index(tmpl, overlay)
				dumpIdx := strings.Index(tmpl, "YAML.dump(values)")
				Expect(overlayIdx).To(BeNumerically("<", dumpIdx), "overlay must appear before YAML.dump")
			})

			It("produces valid output with empty overlay", func() {
				tmpl := generateManifestTemplate("test-install", "")
				Expect(tmpl).To(ContainSubstring("YAML.dump(values)"))
				Expect(tmpl).NotTo(BeEmpty())
			})
		})
	})

	Context("buildRegistryDataSpec", func() {
		It("includes user-declared additional links after cluster-info", func() {
			links := []boshConsumes{
				{Name: "binding_cache", Type: "binding_cache", Optional: false},
			}
			spec, err := buildRegistryDataSpec("", "", links)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec).To(ContainSubstring("name: binding_cache"))
			Expect(spec).To(ContainSubstring("type: binding_cache"))
			Expect(spec).To(ContainSubstring("optional: false"))
			Expect(spec).To(ContainSubstring("name: cluster"))
		})

		It("marks optional links correctly", func() {
			links := []boshConsumes{
				{Name: "optional-link", Type: "some-type", Optional: true},
			}
			spec, err := buildRegistryDataSpec("", "", links)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec).To(ContainSubstring("optional: true"))
		})

		It("includes only cluster-info when no additional links are declared", func() {
			spec, err := buildRegistryDataSpec("", "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec).To(ContainSubstring("name: cluster"))
			Expect(spec).NotTo(ContainSubstring("name: binding_cache"))
		})

		It("safely encodes link names containing YAML-special characters", func() {
			// yaml.Marshal quotes/blocks the value so it cannot inject extra YAML keys.
			// The real type field ("legit-type") must still appear at the correct level.
			links := []boshConsumes{
				{Name: "name: injected\ntype: evil", Type: "legit-type", Optional: false},
			}
			spec, err := buildRegistryDataSpec("", "", links)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec).To(ContainSubstring("type: legit-type"))
		})

		It("emits each unique link name only once given pre-deduplicated input", func() {
			links := []boshConsumes{
				{Name: "binding_cache", Type: "binding_cache", Optional: false},
			}
			spec, err := buildRegistryDataSpec("", "", links)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Count(spec, "name: binding_cache")).To(Equal(1))
		})
	})

	Context("deduplicateConsumes", func() {
		var progressBuf strings.Builder
		var b *baker

		BeforeEach(func() {
			progressBuf.Reset()
			b = &baker{progressWriter: &progressBuf}
		})

		It("keeps all entries when names are unique", func() {
			input := []boshLinkConsumer{
				{Name: "link-a", Type: "type-a"},
				{Name: "link-b", Type: "type-b"},
			}
			result := b.deduplicateConsumes(input)
			Expect(result).To(HaveLen(2))
			Expect(progressBuf.String()).To(BeEmpty())
		})

		It("silently drops exact duplicates without warning", func() {
			input := []boshLinkConsumer{
				{Name: "link-a", Type: "type-a", Optional: false},
				{Name: "link-a", Type: "type-a", Optional: false},
			}
			result := b.deduplicateConsumes(input)
			Expect(result).To(HaveLen(1))
			Expect(progressBuf.String()).To(BeEmpty())
		})

		It("warns and keeps first when conflicting type definitions are found", func() {
			input := []boshLinkConsumer{
				{Name: "binding_cache", Type: "binding_cache"},
				{Name: "binding_cache", Type: "binding-cache-v2"},
			}
			result := b.deduplicateConsumes(input)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Type).To(Equal("binding_cache"))
			Expect(progressBuf.String()).To(ContainSubstring("WARNING"))
			Expect(progressBuf.String()).To(ContainSubstring(`"binding_cache"`))
			Expect(progressBuf.String()).To(ContainSubstring("binding-cache-v2"))
		})

		It("warns and keeps first when optional flag differs", func() {
			input := []boshLinkConsumer{
				{Name: "link-a", Type: "type-a", Optional: false},
				{Name: "link-a", Type: "type-a", Optional: true},
			}
			result := b.deduplicateConsumes(input)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Optional).To(BeFalse())
			Expect(progressBuf.String()).To(ContainSubstring("WARNING"))
		})

		It("emits one warning per conflict when multiple entries share a name", func() {
			input := []boshLinkConsumer{
				{Name: "link-a", Type: "type-a"},
				{Name: "link-a", Type: "type-b"},
				{Name: "link-a", Type: "type-c"},
			}
			result := b.deduplicateConsumes(input)
			Expect(result).To(HaveLen(1))
			Expect(result[0].Type).To(Equal("type-a"))
			Expect(strings.Count(progressBuf.String(), "WARNING")).To(Equal(2))
		})

		It("returns nil without panicking when given a nil slice", func() {
			result := b.deduplicateConsumes(nil)
			Expect(result).To(BeNil())
			Expect(progressBuf.String()).To(BeEmpty())
		})
	})

	Context("jobSpecOverlay", func() {
		It("parses a consumes list from YAML", func() {
			content := `
consumes:
- name: binding_cache
  type: binding_cache
  optional: false
`
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte(content), &overlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(overlay.Consumes).To(HaveLen(1))
			Expect(overlay.Consumes[0].Name).To(Equal("binding_cache"))
			Expect(overlay.Consumes[0].Type).To(Equal("binding_cache"))
			Expect(overlay.Consumes[0].Optional).To(BeFalse())
		})

		It("handles an empty consumes list without error", func() {
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte("consumes: []"), &overlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(overlay.Consumes).To(BeEmpty())
		})

		It("handles a missing consumes key without error", func() {
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte("{}"), &overlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(overlay.Consumes).To(BeNil())
		})

		It("returns an error for malformed YAML", func() {
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte("consumes: [\ninvalid"), &overlay)
			Expect(err).To(HaveOccurred())
		})

		It("parses an entry with only from set (no deployment)", func() {
			content := `
consumes:
- name: nats-tls
  type: nats-tls
  optional: false
  from: nats-tls
`
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte(content), &overlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(overlay.Consumes[0].From).To(Equal("nats-tls"))
			Expect(overlay.Consumes[0].Deployment).To(BeEmpty())
		})

		It("parses an entry with only deployment set (no from)", func() {
			content := `
consumes:
- name: nats-tls
  type: nats-tls
  optional: false
  deployment: "(( ..cf.deployment_name ))"
`
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte(content), &overlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(overlay.Consumes[0].From).To(BeEmpty())
			Expect(overlay.Consumes[0].Deployment).To(Equal("(( ..cf.deployment_name ))"))
		})

		It("parses from and deployment fields for cross-deployment link resolution", func() {
			content := `
consumes:
- name: nats-tls
  type: nats-tls
  optional: false
  from: nats-tls
  deployment: "(( ..cf.deployment_name ))"
`
			var overlay jobSpecOverlay
			err := yaml.Unmarshal([]byte(content), &overlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(overlay.Consumes).To(HaveLen(1))
			c := overlay.Consumes[0]
			Expect(c.Name).To(Equal("nats-tls"))
			Expect(c.From).To(Equal("nats-tls"))
			Expect(c.Deployment).To(Equal("(( ..cf.deployment_name ))"))
		})
	})

	Context("BakeFromLockfile validation", func() {
		When("the release lock name does not match", func() {
			It("returns an error", func() {
				inputPath, err := os.MkdirTemp("", "lockfile-mismatch-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				defer func() { _ = os.RemoveAll(filepath.Dir(inputPath)) }()

				err = os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				releaseLock := cargo.BOSHReleaseTarballLock{
					Name:    "wrong-name",
					Version: "0.1.1",
				}

				subject := NewBaker()
				err = subject.BakeFromLockfile(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, releaseLock, "/nonexistent/tarball.tgz", BakeOptions{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("does not match tile-derived name"))
			})
		})

		When("the tile metadata name is missing", func() {
			It("returns an error early", func() {
				inputPath, err := os.MkdirTemp("", "missing-name-*")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.RemoveAll(inputPath) }()

				err = os.WriteFile(filepath.Join(inputPath, "base.yml"), []byte("label: no-name-tile"), 0644)
				Expect(err).NotTo(HaveOccurred())

				subject := NewBaker()
				err = subject.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required field 'name'"))

				err = subject.BakeFromLockfile(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, cargo.BOSHReleaseTarballLock{}, "/nonexistent/tarball.tgz", BakeOptions{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required field 'name'"))
			})
		})
	})

	Context("Bake validation", func() {
		When("the tile metadata version is too old", func() {
			It("returns an error before invoking bosh", func() {
				inputPath, err := os.MkdirTemp("", "old-metadata-version-*")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.RemoveAll(inputPath) }()

				baseYML := `---
name: k8s-tile-test
label: test tile
metadata_version: "3.1.0"
product_version: "0.1.0"
`
				Expect(os.WriteFile(filepath.Join(inputPath, "base.yml"), []byte(baseYML), 0644)).To(Succeed())

				subject := NewBaker()
				err = subject.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tile metadata_version too old"))
			})
		})
	})

	Context("GetReleaseTarball", func() {
		When("called before bake", func() {
			It("returns an error", func() {
				subject := NewBaker()
				_, err := subject.GetReleaseTarball()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("buildReleaseVersion", func() {
		It("appends fingerprint with + separator", func() {
			Expect(buildReleaseVersion("10.4.0", "a1b2c3d4e5f6")).To(Equal("10.4.0+a1b2c3d4e5f6"))
		})

		It("appends fingerprint with . separator when version already contains +", func() {
			Expect(buildReleaseVersion("10.4.0+beta.1", "a1b2c3d4e5f6")).To(Equal("10.4.0+beta.1.a1b2c3d4e5f6"))
		})
	})

	Context("GetReleaseVersion", func() {
		It("returns empty string before Bake is called", func() {
			subject := NewBaker()
			Expect(subject.GetReleaseVersion()).To(BeEmpty())
		})
	})

	Context("GetBoshReleaseName", func() {
		It("returns empty string before Bake is called", func() {
			b := NewBaker()
			Expect(b.GetBoshReleaseName()).To(BeEmpty())
		})
	})

	Context("generateManifestTemplate with different entry names", func() {
		It("parameterizes the entry name throughout the template", func() {
			template := generateManifestTemplate("my-custom-pkg", "")

			Expect(template).To(ContainSubstring(`p("my-custom-pkg.name")`))
			Expect(template).To(ContainSubstring(`p("my-custom-pkg.version")`))
			Expect(template).To(ContainSubstring(`p("my-custom-pkg.values")`))
			Expect(template).NotTo(ContainSubstring("test-install"))
		})

		It("contains exactly 6 K8s resource documents", func() {
			template := generateManifestTemplate("pkg", "")
			docs := strings.Split(template, "---")
			nonEmpty := 0
			for _, doc := range docs {
				if strings.TrimSpace(doc) != "" {
					nonEmpty++
				}
			}
			Expect(nonEmpty).To(Equal(5))
		})
	})

	Context("validateVariables", func() {
		It("passes for an empty list", func() {
			err := validateVariables([]proofing.Variable{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("passes for a valid certificate variable", func() {
			err := validateVariables([]proofing.Variable{
				{
					Name: "/cf/diego-instance-identity-root-ca-2-6",
					Type: "certificate",
					Options: map[string]any{
						"common_name": "Diego Instance Identity Root CA",
						"is_ca":       true,
						"duration":    1095,
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("errors when name is empty", func() {
			err := validateVariables([]proofing.Variable{
				{Name: "", Type: "certificate"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing required field 'name'"))
		})

		It("errors when type is empty", func() {
			err := validateVariables([]proofing.Variable{
				{Name: "my-var", Type: ""},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing required field 'type'"))
		})
	})
})

func TestShellQuoteCommand(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "single word",
			command: "/var/vcap/jobs/smoke_tests/bin/run",
			want:    `'/var/vcap/jobs/smoke_tests/bin/run'`,
		},
		{
			name:    "command with args",
			command: "/var/vcap/jobs/smoke_tests/bin/run --foo bar",
			want:    `'/var/vcap/jobs/smoke_tests/bin/run' '--foo' 'bar'`,
		},
		{
			name:    "a hostile command is neutralized, not interpreted",
			command: "/bin/true; rm -rf /",
			want:    `'/bin/true;' 'rm' '-rf' '/'`,
		},
		{
			name:    "command substitution is neutralized",
			command: "/bin/true $(whoami) `whoami`",
			want:    "'/bin/true' '$(whoami)' '`whoami`'",
		},
		{
			name:    "embedded single quote is escaped, not closed early",
			command: `/bin/echo it's`,
			want:    `'/bin/echo' 'it'"'"'s'`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuoteCommand(tt.command)
			if got != tt.want {
				t.Errorf("shellQuoteCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
