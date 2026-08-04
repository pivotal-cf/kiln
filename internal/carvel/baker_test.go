package carvel

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/proofing"
	"gopkg.in/yaml.v3"
)

func copyTestFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

func fileChecksum(path string) string {
	f, err := os.Open(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	defer func() { _ = f.Close() }()
	h := sha256.New()
	_, err = io.Copy(h, f)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return hex.EncodeToString(h.Sum(nil))
}

func boshInstalled() bool {
	_, err := exec.LookPath("bosh")
	return err == nil
}

func kilnInstalled() bool {
	_, err := exec.LookPath("kiln")
	return err == nil
}

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

	Context("Bake", func() {
		When("the input directory contains k8s tile data", func() {
			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
			})
			var (
				inputPath, outputPath, boshReleasePath string
				subject                                Baker
				err                                    error
			)
			BeforeEach(func() {
				var err error
				inputPath, err = os.MkdirTemp("", "testinput-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				outputPath = path.Join(inputPath, ".carvel-tile")
				boshReleasePath = path.Join(inputPath, ".boshrelease")
				err = os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())
				// create an initial git commit in the input directory
				commands := []*exec.Cmd{
					exec.Command("git", "init"),
					exec.Command("git", "add", "."),
					exec.Command("git", "commit", "-m", "initial commit"),
				}
				for _, cmd := range commands {
					cmd.Dir = inputPath
					cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
					out, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
				}

				subject = NewBaker()
				subject.SetWriter(GinkgoWriter)
			})
			AfterEach(func() {
				// Clean up the temp directory
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
			})
			JustBeforeEach(func() {
				err = subject.Bake(inputPath)
			})
			When("the tile data is valid", func() {
				JustBeforeEach(func() {
					Expect(err).NotTo(HaveOccurred())
				})
				It("populates the output metadata", func() {
					outMeta := models.MetadataOut{}
					yamlPath := path.Join(outputPath, "base.yml")
					yamlData, err := os.ReadFile(yamlPath)
					Expect(err).NotTo(HaveOccurred())

					err = yaml.Unmarshal(yamlData, &outMeta)
					Expect(err).NotTo(HaveOccurred())

					Expect(outMeta.Name).To(Equal("k8s-tile-test"))
					Expect(subject.GetName()).To(Equal("k8s-tile-test"))
					Expect(subject.GetBoshReleaseName()).To(Equal("k8s-tile-test-pkg"))
					Expect(outMeta.ProductVersion).To(Equal(`$( version )`))
					Expect(outMeta.MetadataVersion).To(Equal("3.2.0"))
					Expect(outMeta.Rank).To(Equal(1))
					Expect(outMeta.Serial).To(BeFalse())
					Expect(outMeta.PropertyBlueprints).To(HaveLen(2))
					Expect(outMeta.FormTypes).To(HaveLen(1))
					Expect(outMeta.Variables).To(HaveLen(1))
					Expect(outMeta.Variables[0].Name).To(Equal("sample-tile-ca"))
					Expect(outMeta.Variables[0].Type).To(Equal("certificate"))
					Expect(outMeta.Variables[0].Options).To(HaveKeyWithValue("common_name", "Sample Tile CA"))
					Expect(outMeta.Variables[0].Options).To(HaveKeyWithValue("is_ca", true))
					Expect(outMeta.Releases).To(HaveLen(1))
					Expect(outMeta.Releases[0]).To(Equal(`$( release "k8s-tile-test-pkg" )`))
					Expect(outMeta.InstanceGroups).To(HaveLen(0))
					Expect(outMeta.RuntimeConfigs).To(HaveLen(1))
					Expect(outMeta.RuntimeConfigs[0]).To(Equal(`$( runtime_config "k8s-tile-test-pkgr" )`))
					Expect(outMeta.CompatibleKubernetesDistributions).To(HaveLen(1))
					Expect(outMeta.CompatibleKubernetesDistributions[0].Name).To(Equal("k0s"))
					Expect(outMeta.CompatibleKubernetesDistributions[0].Version).To(Equal(">0.0.0"))
					Expect(outMeta.RequiresKubernetes).To(BeTrue())
					Expect(outMeta.SupportsParallelDeploys).To(BeTrue())
					Expect(outMeta.RequiresProductVersions).To(HaveLen(1))
					Expect(outMeta.RequiresProductVersions[0].Name).To(Equal("some-other-product"))
					Expect(outMeta.RequiresProductVersions[0].Version).To(Equal(">=1.0.0"))
					Expect(outMeta.RequiresProductVersions[0].Optional).To(BeTrue())
					Expect(outMeta.UsesKubernetesFeatures).To(HaveLen(2))
					Expect(outMeta.UsesKubernetesFeatures[0].Name).To(Equal("gpu-scheduling"))
					Expect(outMeta.UsesKubernetesFeatures[0].Optional).To(BeFalse())
					Expect(outMeta.UsesKubernetesFeatures[1].Name).To(Equal("node-local-storage"))
					Expect(outMeta.UsesKubernetesFeatures[1].Optional).To(BeTrue())
				})
				It("creates empty instance_group and jobs directories", func() {
					Expect(filepath.Join(outputPath, "instance_groups")).To(BeADirectory())
					Expect(filepath.Join(outputPath, "jobs")).To(BeADirectory())
				})
				It("creates a runtime config", func() {
					Expect(filepath.Join(outputPath, "runtime_configs")).To(BeADirectory())
					Expect(filepath.Join(outputPath, "runtime_configs", "k8s-tile-test-pkgr.yml")).To(BeAnExistingFile())
				})
				It("copies forms, properties, icon, version from the input", func() {
					Expect(filepath.Join(outputPath, "properties", "properties.yml")).To(BeAnExistingFile())
					Expect(filepath.Join(outputPath, "forms", "db_props.yml")).To(BeAnExistingFile())
					Expect(filepath.Join(outputPath, "icon.png")).To(BeAnExistingFile())
					Expect(filepath.Join(outputPath, "version")).To(BeAnExistingFile())
				})
				It("generates a bosh release tarball with fingerprinted version", func() {
					releaseVersion := subject.GetReleaseVersion()
					Expect(releaseVersion).To(HavePrefix("0.1.1+"))
					Expect(releaseVersion).To(MatchRegexp(`^0\.1\.1\+[0-9a-f]{12}$`))
					Expect(filepath.Join(outputPath, "releases", "k8s-tile-test-pkg-"+releaseVersion+".tgz")).To(BeAnExistingFile())

					tarball, err := subject.GetReleaseTarball()
					Expect(err).NotTo(HaveOccurred())
					Expect(tarball).To(ContainSubstring(releaseVersion))
				})
				It("does not generate a separate package-install job", func() {
					Expect(filepath.Join(boshReleasePath, "jobs", "package-install")).NotTo(BeADirectory())
				})
				It("generates manifest templates under registry-data job", func() {
					templatePath := filepath.Join(boshReleasePath, "jobs", "registry-data", "templates", "packageinstalls", "test-install.yml.erb")
					Expect(templatePath).To(BeAnExistingFile())

					contents, err := os.ReadFile(templatePath)
					Expect(err).NotTo(HaveOccurred())
					templateStr := string(contents)

					Expect(templateStr).To(ContainSubstring("kind: ServiceAccount"))
					Expect(templateStr).To(ContainSubstring("kind: ClusterRole"))
					Expect(templateStr).To(ContainSubstring("kind: ClusterRoleBinding"))
					Expect(templateStr).To(ContainSubstring("kind: Secret"))
					Expect(templateStr).To(ContainSubstring("kind: PackageInstall"))
					Expect(templateStr).To(ContainSubstring(`link("cluster").p("content-namespace")`))
				})
				It("generates registry-data job spec with BOSH link consumer and templates", func() {
					specPath := filepath.Join(boshReleasePath, "jobs", "registry-data", "spec")
					Expect(specPath).To(BeAnExistingFile())

					contents, err := os.ReadFile(specPath)
					Expect(err).NotTo(HaveOccurred())
					specStr := string(contents)

					Expect(specStr).To(ContainSubstring("name: registry-data"))
					Expect(specStr).To(ContainSubstring("packageinstalls/test-install.yml.erb: packageinstalls/test-install.yml"))
					Expect(specStr).To(ContainSubstring("packages:\n- registry-data"))
					Expect(specStr).To(ContainSubstring("consumes:"))
					Expect(specStr).To(ContainSubstring("name: cluster"))
					Expect(specStr).To(ContainSubstring("type: cluster-info"))
					Expect(specStr).To(ContainSubstring("optional: true"))
					Expect(specStr).To(ContainSubstring("name: binding_cache"))
					Expect(specStr).To(ContainSubstring("type: binding_cache"))
				})
				It("generates runtime config referencing tanzu-content release", func() {
					rcPath := filepath.Join(outputPath, "runtime_configs", "k8s-tile-test-pkgr.yml")
					rcData, err := os.ReadFile(rcPath)
					Expect(err).NotTo(HaveOccurred())

					var rc models.RuntimeConfigOuter
					err = yaml.Unmarshal(rcData, &rc)
					Expect(err).NotTo(HaveOccurred())

					var inner models.RuntimeConfigInner
					err = yaml.Unmarshal([]byte(rc.RuntimeConfig), &inner)
					Expect(err).NotTo(HaveOccurred())

					Expect(inner.Addons).To(HaveLen(1))
					addon := inner.Addons[0]

					By("referencing tanzu-content release instead of registry")
					Expect(addon.Include.Jobs).To(HaveLen(2))
					Expect(addon.Include.Jobs[0].Name).To(Equal("install-package-repository"))
					Expect(addon.Include.Jobs[0].Release).To(Equal("tanzu-content"))
					Expect(addon.Include.Jobs[1].Name).To(Equal("install-packages"))
					Expect(addon.Include.Jobs[1].Release).To(Equal("tanzu-content"))

					By("having only the registry-data job (no separate package-install job)")
					Expect(addon.Jobs).To(HaveLen(1))
					Expect(addon.Jobs[0].Name).To(Equal("registry-data"))
					Expect(addon.Jobs[0].Release).To(Equal("k8s-tile-test-pkg"))

					By("carrying package install properties on the registry-data job")
					Expect(addon.Jobs[0].Properties).To(HaveKey("test-install"))
					props := addon.Jobs[0].Properties["test-install"]
					Expect(props.Name).To(Equal("something-test.tanzu.vmware.com"))
					Expect(props.Version).To(Equal("0.1.5"))

					By("emitting cross-deployment consumes from job-spec-overlay from/deployment fields")
					Expect(addon.Jobs[0].Consumes).To(HaveKey("binding_cache"))
					bc := addon.Jobs[0].Consumes["binding_cache"]
					Expect(bc.From).To(Equal("binding_cache"))
					Expect(bc.Deployment).To(Equal("(( ..cf.deployment_name ))"))
				})
				It("can be kiln baked", func() {
					if !kilnInstalled() {
						Skip("kiln CLI not installed - skipping integration test")
					}
					err := subject.KilnBake(filepath.Join(outputPath, "my-tile.pivotal"))
					Expect(err).NotTo(HaveOccurred())
					Expect(filepath.Join(outputPath, "my-tile.pivotal")).To(BeAnExistingFile())
				})
			})
			When("the tile metadata version is too old", func() {
				BeforeEach(func() {
					m := models.Metadata{
						Name:                     "k8s-tile-test",
						Label:                    "test tile",
						IconImage:                "$( icon )",
						MetadataVersion:          "3.1.0",
						MinimumVersionForUpgrade: "0.0.0",
						ProductVersion:           "$( version )",
						Rank:                     1,
						Serial:                   false,
						PropertyBlueprints: []string{
							`$( property "database_name" )`,
							`$( property "admin_password" )`,
						},
						FormTypes:       []string{`$( form "db_props" )`},
						Variables:       []proofing.Variable{},
						PackageInstalls: []string{`$( package "test-install" )`},
					}
					yamlData, err := yaml.Marshal(&m)
					Expect(err).NotTo(HaveOccurred())
					err = os.WriteFile(path.Join(inputPath, "base.yml"), yamlData, 0644) // 0644 sets file permissions
					Expect(err).NotTo(HaveOccurred())
				})

				It("fails to bake with an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("tile metadata_version too old"))
				})
			})

			When("the tile does not set parallel deploys or product version requirements", func() {
				BeforeEach(func() {
					m := models.Metadata{
						Name:                     "k8s-tile-test",
						Label:                    "test tile",
						IconImage:                "$( icon )",
						MetadataVersion:          "3.2.0",
						MinimumVersionForUpgrade: "0.0.0",
						ProductVersion:           "$( version )",
						Rank:                     1,
						Serial:                   false,
						PropertyBlueprints: []string{
							`$( property "database_name" )`,
							`$( property "admin_password" )`,
						},
						FormTypes:       []string{`$( form "db_props" )`},
						Variables:       []proofing.Variable{},
						PackageInstalls: []string{`$( package "test-install" )`},
					}
					yamlData, err := yaml.Marshal(&m)
					Expect(err).NotTo(HaveOccurred())
					err = os.WriteFile(path.Join(inputPath, "base.yml"), yamlData, 0644) // 0644 sets file permissions
					Expect(err).NotTo(HaveOccurred())
				})

				It("omits both fields from the generated base.yml", func() {
					Expect(err).NotTo(HaveOccurred())

					yamlPath := path.Join(outputPath, "base.yml")
					rawYaml, err := os.ReadFile(yamlPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(rawYaml)).NotTo(ContainSubstring("supports_parallel_deploys"))
					Expect(string(rawYaml)).NotTo(ContainSubstring("requires_product_versions"))

					outMeta := models.MetadataOut{}
					err = yaml.Unmarshal(rawYaml, &outMeta)
					Expect(err).NotTo(HaveOccurred())
					Expect(outMeta.SupportsParallelDeploys).To(BeFalse())
					Expect(outMeta.RequiresProductVersions).To(BeEmpty())
				})
			})
			When("the tile declares no kubernetes features", func() {
				// Ops Manager rejects uses_kubernetes_features on a tile that is not a
				// kubernetes consumer, so an absent block must stay absent rather than
				// baking out as an empty list.
				BeforeEach(func() {
					m := models.Metadata{
						Name:                     "k8s-tile-test",
						Label:                    "test tile",
						IconImage:                "$( icon )",
						MetadataVersion:          "3.2.0",
						MinimumVersionForUpgrade: "0.0.0",
						ProductVersion:           "$( version )",
						Rank:                     1,
						Serial:                   false,
						PropertyBlueprints: []string{
							`$( property "database_name" )`,
							`$( property "admin_password" )`,
						},
						FormTypes:                         []string{`$( form "db_props" )`},
						Variables:                         []proofing.Variable{},
						PackageInstalls:                   []string{`$( package "test-install" )`},
						CompatibleKubernetesDistributions: []models.ProductVersion{{Name: "k0s", Version: ">0.0.0"}},
					}
					yamlData, err := yaml.Marshal(&m)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(yamlData)).NotTo(ContainSubstring("uses_kubernetes_features"))
					err = os.WriteFile(path.Join(inputPath, "base.yml"), yamlData, 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("omits uses_kubernetes_features from the baked metadata", func() {
					Expect(err).NotTo(HaveOccurred())

					yamlData, readErr := os.ReadFile(path.Join(outputPath, "base.yml"))
					Expect(readErr).NotTo(HaveOccurred())
					Expect(string(yamlData)).NotTo(ContainSubstring("uses_kubernetes_features"))

					outMeta := models.MetadataOut{}
					Expect(yaml.Unmarshal(yamlData, &outMeta)).To(Succeed())
					Expect(outMeta.UsesKubernetesFeatures).To(BeEmpty())
					// the rest of the kubernetes metadata is unaffected
					Expect(outMeta.RequiresKubernetes).To(BeTrue())
					Expect(outMeta.CompatibleKubernetesDistributions).To(HaveLen(1))
				})
			})
		})
	})

	Context("BakeFromLockfile", func() {
		When("a valid release lock references a pre-built release", func() {
			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
			})

			It("produces tile output without regenerating the BOSH release", func() {
				inputPath, err := os.MkdirTemp("", "lockfile-test-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				defer func() { _ = os.RemoveAll(filepath.Dir(inputPath)) }()

				err = os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				commands := []*exec.Cmd{
					exec.Command("git", "init"),
					exec.Command("git", "add", "."),
					exec.Command("git", "commit", "-m", "initial commit"),
				}
				for _, cmd := range commands {
					cmd.Dir = inputPath
					cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
					out, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
				}

				subject := NewBaker()
				subject.SetWriter(GinkgoWriter)
				err = subject.Bake(inputPath)
				Expect(err).NotTo(HaveOccurred())

				tarball, err := subject.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())

				cachedTarball := filepath.Join(filepath.Dir(inputPath), "cached-release.tgz")
				err = copyTestFile(tarball, cachedTarball)
				Expect(err).NotTo(HaveOccurred())

				uploadReleaseVersion := subject.GetReleaseVersion()

				releaseLock := cargo.BOSHReleaseTarballLock{
					Name:    "k8s-tile-test-pkg",
					Version: uploadReleaseVersion,
				}

				subject2 := NewBaker()
				subject2.SetWriter(GinkgoWriter)
				err = subject2.BakeFromLockfile(inputPath, releaseLock, cachedTarball)
				Expect(err).NotTo(HaveOccurred())

				outputPath := path.Join(inputPath, ".carvel-tile")
				Expect(filepath.Join(outputPath, "base.yml")).To(BeAnExistingFile())
				Expect(filepath.Join(outputPath, "releases", "k8s-tile-test-pkg-"+uploadReleaseVersion+".tgz")).To(BeAnExistingFile())
				Expect(filepath.Join(outputPath, "runtime_configs")).To(BeADirectory())
				Expect(subject2.GetReleaseVersion()).To(Equal(uploadReleaseVersion))
			})
		})

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
				err = subject.BakeFromLockfile(inputPath, releaseLock, "/nonexistent/tarball.tgz")
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
				err = subject.Bake(inputPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required field 'name'"))

				err = subject.BakeFromLockfile(inputPath, cargo.BOSHReleaseTarballLock{}, "/nonexistent/tarball.tgz")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required field 'name'"))
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

	Context("rebake reproducibility", func() {
		It("publish and rebake produce identical tiles when using the same cached release", func() {
			if !boshInstalled() {
				Skip("bosh CLI not installed")
			}
			if !kilnInstalled() {
				Skip("kiln CLI not installed")
			}

			tmpRoot, err := os.MkdirTemp("", "rebake-repro-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(tmpRoot) }()

			inputPath := filepath.Join(tmpRoot, "tile")
			err = os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))
			Expect(err).NotTo(HaveOccurred())

			for _, cmd := range []*exec.Cmd{
				exec.Command("git", "init"),
				exec.Command("git", "add", "."),
				exec.Command("git", "commit", "-m", "initial commit"),
			} {
				cmd.Dir = inputPath
				out, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "git setup: "+string(out))
			}

			uploadBaker := NewBaker()
			uploadBaker.SetWriter(GinkgoWriter)
			err = uploadBaker.Bake(inputPath)
			Expect(err).NotTo(HaveOccurred())

			uploadTarball, err := uploadBaker.GetReleaseTarball()
			Expect(err).NotTo(HaveOccurred())
			cachedTarball := filepath.Join(tmpRoot, "cached-release.tgz")
			Expect(copyTestFile(uploadTarball, cachedTarball)).To(Succeed())

			releaseLock := cargo.BOSHReleaseTarballLock{
				Name:    "k8s-tile-test-pkg",
				Version: uploadBaker.GetReleaseVersion(),
			}

			publishBaker := NewBaker()
			publishBaker.SetWriter(GinkgoWriter)
			err = publishBaker.BakeFromLockfile(inputPath, releaseLock, cachedTarball)
			Expect(err).NotTo(HaveOccurred())

			publishTile := filepath.Join(tmpRoot, "publish.pivotal")
			err = publishBaker.KilnBake(publishTile)
			Expect(err).NotTo(HaveOccurred())

			publishChecksum := fileChecksum(publishTile)

			rebakeBaker := NewBaker()
			rebakeBaker.SetWriter(GinkgoWriter)
			err = rebakeBaker.BakeFromLockfile(inputPath, releaseLock, cachedTarball)
			Expect(err).NotTo(HaveOccurred())

			rebakeTile := filepath.Join(tmpRoot, "rebake.pivotal")
			err = rebakeBaker.KilnBake(rebakeTile)
			Expect(err).NotTo(HaveOccurred())

			rebakeChecksum := fileChecksum(rebakeTile)

			Expect(rebakeChecksum).To(Equal(publishChecksum),
				"publish and rebake should produce identical tiles when using the same cached BOSH release tarball")
		})
	})

	Context("hashBoshReleaseInputs", func() {
		It("is deterministic", func() {
			dir, err := os.MkdirTemp("", "hash-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(dir) }()

			Expect(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(dir, "sub"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0644)).To(Succeed())

			h1, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())
			h2, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(h1).To(Equal(h2))
			Expect(h1).To(HaveLen(12))
		})

		It("changes when file contents change", func() {
			dir, err := os.MkdirTemp("", "hash-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(dir) }()

			Expect(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)).To(Succeed())

			h1, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0644)).To(Succeed())

			h2, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(h1).NotTo(Equal(h2))
		})

		It("changes when a file is renamed", func() {
			dir, err := os.MkdirTemp("", "hash-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(dir) }()

			Expect(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)).To(Succeed())

			h1, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt"))).To(Succeed())

			h2, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(h1).NotTo(Equal(h2))
		})

		It("excludes .git directory", func() {
			dir, err := os.MkdirTemp("", "hash-test-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(dir) }()

			Expect(os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)).To(Succeed())

			h1, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/other"), 0644)).To(Succeed())

			h2, err := hashBoshReleaseInputs(dir)
			Expect(err).NotTo(HaveOccurred())

			Expect(h1).To(Equal(h2))
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
