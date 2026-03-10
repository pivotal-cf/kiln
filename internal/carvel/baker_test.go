package carvel

import (
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/carvel/models"
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
			template = generateManifestTemplate("test-install")
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
					Expect(outMeta.ProductVersion).To(Equal(`$( version )`))
					Expect(outMeta.MetadataVersion).To(Equal("3.2.0"))
					Expect(outMeta.Rank).To(Equal(1))
					Expect(outMeta.Serial).To(BeFalse())
					Expect(outMeta.PropertyBlueprints).To(HaveLen(2))
					Expect(outMeta.FormTypes).To(HaveLen(1))
					Expect(outMeta.Variables).To(BeEmpty())
					Expect(outMeta.Releases).To(HaveLen(1))
					Expect(outMeta.Releases[0]).To(ContainSubstring("k8s-tile-test"))
					Expect(outMeta.InstanceGroups).To(HaveLen(0))
					Expect(outMeta.RuntimeConfigs).To(HaveLen(1))
					Expect(outMeta.RuntimeConfigs[0]).To(Equal(`$( runtime_config "k8s-tile-test-pkgr" )`))
					Expect(outMeta.CompatibleKubernetesDistributions).To(HaveLen(1))
					Expect(outMeta.CompatibleKubernetesDistributions[0].Name).To(Equal("k0s"))
					Expect(outMeta.CompatibleKubernetesDistributions[0].Version).To(Equal(">0.0.0"))
					Expect(outMeta.RequiresKubernetes).To(BeTrue())
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
				It("Generates a bosh release tarball", func() {
					Expect(filepath.Join(outputPath, "releases", "k8s-tile-test-0.1.1.tgz")).To(BeAnExistingFile())
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
					Expect(addon.Jobs[0].Release).To(Equal("k8s-tile-test"))

					By("carrying package install properties on the registry-data job")
					Expect(addon.Jobs[0].Properties).To(HaveKey("test-install"))
					props := addon.Jobs[0].Properties["test-install"]
					Expect(props.Name).To(Equal("something-test.tanzu.vmware.com"))
					Expect(props.Version).To(Equal("0.1.5"))
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
						Variables:       []string{},
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
		})
	})

	Context("BakeFromLockfile", func() {
		When("a valid lockfile references a pre-built release", func() {
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
					out, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
				}

				// First do a normal bake to produce a real BOSH release tarball
				subject := NewBaker()
				subject.SetWriter(GinkgoWriter)
				err = subject.Bake(inputPath)
				Expect(err).NotTo(HaveOccurred())

				tarball, err := subject.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())

				// Copy the tarball to a temp location (simulating Artifactory cache)
				cachedTarball := filepath.Join(filepath.Dir(inputPath), "cached-release.tgz")
				err = copyTestFile(tarball, cachedTarball)
				Expect(err).NotTo(HaveOccurred())

				// Write a lockfile pointing to the cached tarball
				lf := models.CarvelLockfile{
					Release: models.CarvelReleaseLock{
						Name:       "k8s-tile-test",
						Version:    "0.1.1",
						RemotePath: cachedTarball,
						SHA256:     "test-sha",
					},
				}
				lockfilePath := filepath.Join(filepath.Dir(inputPath), "Kilnfile.lock")
				err = lf.WriteFile(lockfilePath)
				Expect(err).NotTo(HaveOccurred())

				// Now bake from lockfile
				subject2 := NewBaker()
				subject2.SetWriter(GinkgoWriter)
				err = subject2.BakeFromLockfile(inputPath, lockfilePath)
				Expect(err).NotTo(HaveOccurred())

				outputPath := path.Join(inputPath, ".carvel-tile")
				Expect(filepath.Join(outputPath, "base.yml")).To(BeAnExistingFile())
				Expect(filepath.Join(outputPath, "releases", "k8s-tile-test-0.1.1.tgz")).To(BeAnExistingFile())
				Expect(filepath.Join(outputPath, "runtime_configs")).To(BeADirectory())
			})
		})

		When("the lockfile release name does not match", func() {
			It("returns an error", func() {
				inputPath, err := os.MkdirTemp("", "lockfile-mismatch-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				defer func() { _ = os.RemoveAll(filepath.Dir(inputPath)) }()

				err = os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				lf := models.CarvelLockfile{
					Release: models.CarvelReleaseLock{
						Name:    "wrong-name",
						Version: "0.1.1",
					},
				}
				lockfilePath := filepath.Join(filepath.Dir(inputPath), "Kilnfile.lock")
				err = lf.WriteFile(lockfilePath)
				Expect(err).NotTo(HaveOccurred())

				subject := NewBaker()
				err = subject.BakeFromLockfile(inputPath, lockfilePath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("does not match tile name"))
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

	Context("generateManifestTemplate with different entry names", func() {
		It("parameterizes the entry name throughout the template", func() {
			template := generateManifestTemplate("my-custom-pkg")

			Expect(template).To(ContainSubstring(`p("my-custom-pkg.name")`))
			Expect(template).To(ContainSubstring(`p("my-custom-pkg.version")`))
			Expect(template).To(ContainSubstring(`p("my-custom-pkg.values")`))
			Expect(template).NotTo(ContainSubstring("test-install"))
		})

		It("contains exactly 6 K8s resource documents", func() {
			template := generateManifestTemplate("pkg")
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
})
