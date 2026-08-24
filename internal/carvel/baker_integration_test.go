//go:build integration

package carvel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/proofing"
)

// readReleaseManifest extracts and parses release.MF from a bosh release
// tarball, for assertions on fields (like commit_hash) that canonicalization
// deliberately leaves untouched.
func readReleaseManifest(tarballPath string) (manifestDoc, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return manifestDoc{}, err
	}
	defer func() { _ = f.Close() }()

	spoolDir, err := os.MkdirTemp("", "read-manifest-*")
	if err != nil {
		return manifestDoc{}, err
	}
	defer func() { _ = os.RemoveAll(spoolDir) }()

	entries, _, err := readTarGz(f, spoolDir)
	if err != nil {
		return manifestDoc{}, err
	}
	entry, ok := entries[manifestFileName]
	if !ok {
		return manifestDoc{}, fmt.Errorf("release.MF not found in %s", tarballPath)
	}
	data, err := os.ReadFile(entry.path)
	if err != nil {
		return manifestDoc{}, err
	}
	var doc manifestDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return manifestDoc{}, err
	}
	return doc, nil
}

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

// These specs shell out to the real bosh CLI, so they are gated behind the
// integration build tag rather than a runtime PATH check: the unit suite stays
// hermetic, and `go test -tags integration` fails loudly if bosh is missing.
var _ = Describe("Carvel Baker (integration)", func() {
	Context("Bake", func() {
		When("the input directory contains k8s tile data", func() {
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
			var (
				kilnfile     cargo.Kilnfile
				kilnfileLock cargo.KilnfileLock
				opts         BakeOptions
			)

			BeforeEach(func() {
				kilnfile = cargo.Kilnfile{}
				kilnfileLock = cargo.KilnfileLock{}
				opts = BakeOptions{}
			})

			JustBeforeEach(func() {
				err = subject.Bake(inputPath, kilnfile, kilnfileLock, opts)
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
					propsRaw := addon.Jobs[0].Properties["test-install"]
					propsMap, ok := propsRaw.(map[string]interface{})
					Expect(ok).To(BeTrue(), "expected PackageInstallProps to unmarshal as map[string]interface{}")
					Expect(propsMap).To(HaveKeyWithValue("name", "something-test.tanzu.vmware.com"))
					Expect(propsMap).To(HaveKeyWithValue("version", "0.1.5"))

					By("emitting cross-deployment consumes from job-spec-overlay from/deployment fields")
					Expect(addon.Jobs[0].Consumes).To(HaveKey("binding_cache"))
					bc := addon.Jobs[0].Consumes["binding_cache"]
					Expect(bc.From).To(Equal("binding_cache"))
					Expect(bc.Deployment).To(Equal("(( ..cf.deployment_name ))"))
				})
				It("can be kiln baked", func() {
					err := subject.KilnBake(filepath.Join(outputPath, "my-tile.pivotal"))
					Expect(err).NotTo(HaveOccurred())
					Expect(filepath.Join(outputPath, "my-tile.pivotal")).To(BeAnExistingFile())
				})

				When("the tile base.yml declares additional_releases", func() {
					const (
						extraReleaseName    = "smoke-test-scripts"
						extraReleaseVersion = "dev"
						extraJobName        = "smoke-test-scripts"
					)

					BeforeEach(func() {
						baseYMLPath := filepath.Join(inputPath, "base.yml")
						raw, err := os.ReadFile(baseYMLPath)
						Expect(err).NotTo(HaveOccurred())
						var m models.Metadata
						Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
						m.AdditionalReleases = []models.AdditionalRelease{
							{
								Name: extraReleaseName,
								Jobs: []models.AdditionalJob{{Name: extraJobName}},
							},
						}
						updated, err := yaml.Marshal(&m)
						Expect(err).NotTo(HaveOccurred())
						Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())

						kilnfile = cargo.Kilnfile{
							ReleaseSources: []cargo.ReleaseSourceConfig{
								{Type: "s3", Bucket: "fake-bucket", Region: "us-west-1", PathTemplate: "fake-path-template"},
							},
						}
						kilnfileLock = cargo.KilnfileLock{
							Releases: []cargo.BOSHReleaseTarballLock{
								{Name: extraReleaseName, Version: extraReleaseVersion, RemoteSource: "fake-bucket"},
							},
						}
						opts = BakeOptions{
							SkipFetch:         true,
							ReleasesDirectory: filepath.Join(inputPath, "releases"),
						}

						Expect(os.MkdirAll(opts.ReleasesDirectory, 0755)).To(Succeed())
						stubTarball := filepath.Join(opts.ReleasesDirectory, extraReleaseName+"-"+extraReleaseVersion+".tgz")
						Expect(os.WriteFile(stubTarball, []byte("stub bosh release tarball"), 0644)).To(Succeed())
					})

					It("copies the tarball verbatim into .carvel-tile/releases/ when skip-fetch is true", func() {
						dst := filepath.Join(outputPath, "releases", extraReleaseName+"-"+extraReleaseVersion+".tgz")
						Expect(dst).To(BeAnExistingFile())
						data, err := os.ReadFile(dst)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(data)).To(Equal("stub bosh release tarball"))
					})

					It("fails when skip-fetch is false and tarball is not in cache", func() {
						opts.SkipFetch = false
						_ = os.Remove(filepath.Join(opts.ReleasesDirectory, extraReleaseName+"-"+extraReleaseVersion+".tgz"))
						err := subject.Bake(inputPath, kilnfile, kilnfileLock, opts)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("failed to download additional release"))
					})

					It("adds the additional release to the top-level tile releases list", func() {
						baseYMLPath := filepath.Join(outputPath, "base.yml")
						raw, err := os.ReadFile(baseYMLPath)
						Expect(err).NotTo(HaveOccurred())
						var outMeta models.MetadataOut
						Expect(yaml.Unmarshal(raw, &outMeta)).To(Succeed())
						Expect(outMeta.Releases).To(HaveLen(2))
						Expect(outMeta.Releases).To(ContainElement(ContainSubstring(extraReleaseName)))
					})

					It("extends the runtime-config addon with the extra release and job", func() {
						rcPath := filepath.Join(outputPath, "runtime_configs", "k8s-tile-test-pkgr.yml")
						rcData, err := os.ReadFile(rcPath)
						Expect(err).NotTo(HaveOccurred())
						var rc models.RuntimeConfigOuter
						Expect(yaml.Unmarshal(rcData, &rc)).To(Succeed())
						var inner models.RuntimeConfigInner
						Expect(yaml.Unmarshal([]byte(rc.RuntimeConfig), &inner)).To(Succeed())

						By("adding the release to the releases: list")
						Expect(inner.Releases).To(ContainElement(`$( release "` + extraReleaseName + `" )`))

						By("appending the job to the addon's jobs: list")
						addon := inner.Addons[0]
						Expect(addon.Jobs).To(HaveLen(2))
						extraJob := addon.Jobs[1]
						Expect(extraJob.Name).To(Equal(extraJobName))
						Expect(extraJob.Release).To(Equal(extraReleaseName))
						Expect(extraJob.Properties).To(BeEmpty())
					})
				})

				When("an additional release's RemotePath basename differs from <name>-<version>.tgz", func() {
					const (
						extraReleaseName    = "smoke-test-scripts"
						extraReleaseVersion = "4.12.14"
						extraJobName        = "smoke-test-scripts"
					)
					realFilename := extraReleaseName + "-" + extraReleaseVersion + "-ubuntu-jammy-1.1298.tgz"

					BeforeEach(func() {
						baseYMLPath := filepath.Join(inputPath, "base.yml")
						raw, err := os.ReadFile(baseYMLPath)
						Expect(err).NotTo(HaveOccurred())
						var m models.Metadata
						Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
						m.AdditionalReleases = []models.AdditionalRelease{
							{
								Name: extraReleaseName,
								Jobs: []models.AdditionalJob{{Name: extraJobName}},
							},
						}
						updated, err := yaml.Marshal(&m)
						Expect(err).NotTo(HaveOccurred())
						Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())

						kilnfile = cargo.Kilnfile{
							ReleaseSources: []cargo.ReleaseSourceConfig{
								{Type: "s3", Bucket: "fake-bucket", Region: "us-west-1", PathTemplate: "fake-path-template"},
							},
						}
						kilnfileLock = cargo.KilnfileLock{
							Releases: []cargo.BOSHReleaseTarballLock{
								{
									Name:         extraReleaseName,
									Version:      extraReleaseVersion,
									RemoteSource: "fake-bucket",
									RemotePath:   extraReleaseName + "/" + realFilename,
								},
							},
						}
						opts = BakeOptions{
							SkipFetch:         true,
							ReleasesDirectory: filepath.Join(inputPath, "releases"),
						}

						Expect(os.MkdirAll(opts.ReleasesDirectory, 0755)).To(Succeed())
						stubTarball := filepath.Join(opts.ReleasesDirectory, realFilename)
						Expect(os.WriteFile(stubTarball, []byte("stub smoke-tests tarball"), 0644)).To(Succeed())
					})

					It("finds the tarball under its real filename with --skip-fetch", func() {
						Expect(err).NotTo(HaveOccurred())
						dst := filepath.Join(outputPath, "releases", extraReleaseName+"-"+extraReleaseVersion+".tgz")
						Expect(dst).To(BeAnExistingFile())
						data, readErr := os.ReadFile(dst)
						Expect(readErr).NotTo(HaveOccurred())
						Expect(string(data)).To(Equal("stub smoke-tests tarball"))
					})
				})

				When("an additional_release job carries a properties block", func() {
					const (
						extraReleaseName    = "smoke-tests"
						extraReleaseVersion = "dev"
					)

					BeforeEach(func() {
						baseYMLPath := filepath.Join(inputPath, "base.yml")
						raw, err := os.ReadFile(baseYMLPath)
						Expect(err).NotTo(HaveOccurred())
						var m models.Metadata
						Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
						m.AdditionalReleases = []models.AdditionalRelease{
							{
								Name: extraReleaseName,
								Jobs: []models.AdditionalJob{
									{Name: "dummy-smoke-tests"},
									{
										Name: "smoke_tests",
										Properties: map[string]interface{}{
											"bpm": map[string]interface{}{"enabled": false},
											"smoke_tests": map[string]interface{}{
												"api": "(( .properties.smoke_tests_api.value ))",
											},
										},
									},
								},
							},
						}
						updated, err := yaml.Marshal(&m)
						Expect(err).NotTo(HaveOccurred())
						Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())

						kilnfile = cargo.Kilnfile{
							ReleaseSources: []cargo.ReleaseSourceConfig{
								{Type: "s3", Bucket: "fake-bucket", Region: "us-west-1", PathTemplate: "fake-path-template"},
							},
						}
						kilnfileLock = cargo.KilnfileLock{
							Releases: []cargo.BOSHReleaseTarballLock{
								{Name: extraReleaseName, Version: extraReleaseVersion, RemoteSource: "fake-bucket"},
							},
						}
						opts = BakeOptions{
							SkipFetch:         true,
							ReleasesDirectory: filepath.Join(inputPath, "releases"),
						}

						Expect(os.MkdirAll(opts.ReleasesDirectory, 0755)).To(Succeed())
						stubTarball := filepath.Join(opts.ReleasesDirectory, extraReleaseName+"-"+extraReleaseVersion+".tgz")
						Expect(os.WriteFile(stubTarball, []byte("stub smoke-tests tarball"), 0644)).To(Succeed())
					})

					It("marshals per-job properties through to the generated runtime-config", func() {
						rcPath := filepath.Join(outputPath, "runtime_configs", "k8s-tile-test-pkgr.yml")
						rcData, err := os.ReadFile(rcPath)
						Expect(err).NotTo(HaveOccurred())
						var rc models.RuntimeConfigOuter
						Expect(yaml.Unmarshal(rcData, &rc)).To(Succeed())
						var inner models.RuntimeConfigInner
						Expect(yaml.Unmarshal([]byte(rc.RuntimeConfig), &inner)).To(Succeed())

						addon := inner.Addons[0]
						Expect(addon.Jobs).To(HaveLen(3))

						By("keeping the no-properties job empty")
						dummyJob := addon.Jobs[1]
						Expect(dummyJob.Name).To(Equal("dummy-smoke-tests"))
						Expect(dummyJob.Release).To(Equal(extraReleaseName))
						Expect(dummyJob.Properties).To(BeEmpty())

						By("round-tripping the nested properties block intact")
						smokeJob := addon.Jobs[2]
						Expect(smokeJob.Name).To(Equal("smoke_tests"))
						Expect(smokeJob.Release).To(Equal(extraReleaseName))
						smokeProps, ok := smokeJob.Properties["smoke_tests"].(map[string]interface{})
						Expect(ok).To(BeTrue(), "smoke_tests key should be a nested map")
						Expect(smokeProps["api"]).To(Equal("(( .properties.smoke_tests_api.value ))"))
						bpmProps, ok := smokeJob.Properties["bpm"].(map[string]interface{})
						Expect(ok).To(BeTrue(), "bpm key should be a nested map")
						Expect(bpmProps["enabled"]).To(BeFalse())
					})
				})

				When("the tile base.yml declares post_install_hooks", func() {
					BeforeEach(func() {
						baseYMLPath := filepath.Join(inputPath, "base.yml")
						raw, err := os.ReadFile(baseYMLPath)
						Expect(err).NotTo(HaveOccurred())
						var m models.Metadata
						Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
						m.PostInstallHooks = []models.HookDeclaration{
							{
								Name:    "smoke-tests-post-install-hook",
								Command: "/var/vcap/jobs/smoke_tests/bin/run",
							},
						}
						updated, err := yaml.Marshal(&m)
						Expect(err).NotTo(HaveOccurred())
						Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())
					})

					It("synthesizes the hook adapter job into the auto-generated release", func() {
						releaseVersion := subject.GetReleaseVersion()
						tarballPath := filepath.Join(outputPath, "releases", "k8s-tile-test-pkg-"+releaseVersion+".tgz")
						Expect(tarballPath).To(BeAnExistingFile())

						jobDir := filepath.Join(boshReleasePath, "jobs", "k8s-tile-test-smoke-tests-post-install-hook")
						Expect(jobDir).To(BeADirectory())

						specPath := filepath.Join(jobDir, "spec")
						Expect(specPath).To(BeAnExistingFile())
						specData, err := os.ReadFile(specPath)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(specData)).To(ContainSubstring("name: k8s-tile-test-smoke-tests-post-install-hook"))
						Expect(string(specData)).To(ContainSubstring("hooks-post-install.erb: bin/hooks/post-install"))

						templatePath := filepath.Join(jobDir, "templates", "hooks-post-install.erb")
						Expect(templatePath).To(BeAnExistingFile())
						templateData, err := os.ReadFile(templatePath)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(templateData)).To(ContainSubstring("exec '/var/vcap/jobs/smoke_tests/bin/run'"))
					})

					It("extends the runtime-config addon with the synthesized job", func() {
						rcPath := filepath.Join(outputPath, "runtime_configs", "k8s-tile-test-pkgr.yml")
						rcData, err := os.ReadFile(rcPath)
						Expect(err).NotTo(HaveOccurred())
						var rc models.RuntimeConfigOuter
						Expect(yaml.Unmarshal(rcData, &rc)).To(Succeed())
						var inner models.RuntimeConfigInner
						Expect(yaml.Unmarshal([]byte(rc.RuntimeConfig), &inner)).To(Succeed())

						addon := inner.Addons[0]
						Expect(addon.Jobs).To(HaveLen(2))
						hookJob := addon.Jobs[1]
						Expect(hookJob.Name).To(Equal("k8s-tile-test-smoke-tests-post-install-hook"))
						Expect(hookJob.Release).To(Equal("k8s-tile-test-pkg"))
					})
				})

				Context("when Kilnfile.lock is present alongside hook declarations", func() {
					BeforeEach(func() {
						baseYMLPath := filepath.Join(inputPath, "base.yml")
						raw, err := os.ReadFile(baseYMLPath)
						Expect(err).NotTo(HaveOccurred())
						var m models.Metadata
						Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
						m.PostInstallHooks = []models.HookDeclaration{
							{
								Name:    "smoke-tests-post-install-hook",
								Command: "/var/vcap/jobs/smoke_tests/bin/run",
							},
						}
						updated, err := yaml.Marshal(&m)
						Expect(err).NotTo(HaveOccurred())
						Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())

						lockData := `---
releases:
- name: some-release
  version: "1.2.3"
  sha1: some-sha
`
						err = os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), []byte(lockData), 0644)
						Expect(err).NotTo(HaveOccurred())
					})

					It("still synthesizes the registry-data and hook adapter jobs into the auto-generated release", func() {
						jobDir := filepath.Join(boshReleasePath, "jobs", "k8s-tile-test-smoke-tests-post-install-hook")
						Expect(jobDir).To(BeADirectory())

						specPath := filepath.Join(jobDir, "spec")
						Expect(specPath).To(BeAnExistingFile())

						registryDataDir := filepath.Join(boshReleasePath, "jobs", "registry-data")
						Expect(registryDataDir).To(BeADirectory())
					})
				})
			})

			When("a hook declaration is missing name or command", func() {
				BeforeEach(func() {
					baseYMLPath := filepath.Join(inputPath, "base.yml")
					raw, err := os.ReadFile(baseYMLPath)
					Expect(err).NotTo(HaveOccurred())
					var m models.Metadata
					Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
					m.PostInstallHooks = []models.HookDeclaration{
						{Name: "missing-command"},
					}
					updated, err := yaml.Marshal(&m)
					Expect(err).NotTo(HaveOccurred())
					Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())
				})

				It("returns a clear error", func() {
					Expect(err).To(MatchError(ContainSubstring("post-install hook declaration missing name or command")))
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
					err = os.WriteFile(path.Join(inputPath, "base.yml"), yamlData, 0644)
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
				err = subject.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})
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
				err = subject2.BakeFromLockfile(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, releaseLock, cachedTarball, BakeOptions{})
				Expect(err).NotTo(HaveOccurred())

				outputPath := path.Join(inputPath, ".carvel-tile")
				Expect(filepath.Join(outputPath, "base.yml")).To(BeAnExistingFile())
				Expect(filepath.Join(outputPath, "releases", "k8s-tile-test-pkg-"+uploadReleaseVersion+".tgz")).To(BeAnExistingFile())
				Expect(filepath.Join(outputPath, "runtime_configs")).To(BeADirectory())
				Expect(subject2.GetReleaseVersion()).To(Equal(uploadReleaseVersion))
			})
		})

		When("a hook declaration is missing name or command", func() {
			It("returns a clear error instead of generating a broken runtime-config addon job", func() {
				inputPath, err := os.MkdirTemp("", "lockfile-bad-hook-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				defer func() { _ = os.RemoveAll(filepath.Dir(inputPath)) }()

				err = os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				baseYMLPath := filepath.Join(inputPath, "base.yml")
				raw, err := os.ReadFile(baseYMLPath)
				Expect(err).NotTo(HaveOccurred())
				var m models.Metadata
				Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
				m.PostInstallHooks = []models.HookDeclaration{
					{Name: "missing-command"},
				}
				updated, err := yaml.Marshal(&m)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())

				releaseLock := cargo.BOSHReleaseTarballLock{
					Name:    "k8s-tile-test-pkg",
					Version: "0.1.1",
				}

				subject := NewBaker()
				err = subject.BakeFromLockfile(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, releaseLock, "/nonexistent/tarball.tgz", BakeOptions{})
				Expect(err).To(MatchError(ContainSubstring("post-install hook declaration missing name or command")))
			})
		})
	})

	Context("rebake reproducibility", func() {
		It("publish and rebake produce identical tiles when using the same cached release", func() {

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
				cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
				out, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "git setup: "+string(out))
			}

			uploadBaker := NewBaker()
			uploadBaker.SetWriter(GinkgoWriter)
			err = uploadBaker.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})
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
			err = publishBaker.BakeFromLockfile(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, releaseLock, cachedTarball, BakeOptions{})
			Expect(err).NotTo(HaveOccurred())

			publishTile := filepath.Join(tmpRoot, "publish.pivotal")
			err = publishBaker.KilnBake(publishTile)
			Expect(err).NotTo(HaveOccurred())

			publishChecksum := fileChecksum(publishTile)

			rebakeBaker := NewBaker()
			rebakeBaker.SetWriter(GinkgoWriter)
			err = rebakeBaker.BakeFromLockfile(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, releaseLock, cachedTarball, BakeOptions{})
			Expect(err).NotTo(HaveOccurred())

			rebakeTile := filepath.Join(tmpRoot, "rebake.pivotal")
			err = rebakeBaker.KilnBake(rebakeTile)
			Expect(err).NotTo(HaveOccurred())

			rebakeChecksum := fileChecksum(rebakeTile)

			Expect(rebakeChecksum).To(Equal(publishChecksum),
				"publish and rebake should produce identical tiles when using the same cached BOSH release tarball")
		})
	})

	Context("cold bake reproducibility", func() {
		It("produces byte-identical bosh release tarballs from two independent bakes of the same commit", func() {

			tmpRoot, err := os.MkdirTemp("", "cold-bake-repro-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(tmpRoot) }()

			committedPath := filepath.Join(tmpRoot, "committed")
			err = os.CopyFS(committedPath, os.DirFS("testdata/sample-tile"))
			Expect(err).NotTo(HaveOccurred())

			for _, cmd := range []*exec.Cmd{
				exec.Command("git", "init"),
				exec.Command("git", "add", "."),
				exec.Command("git", "commit", "-m", "initial commit"),
			} {
				cmd.Dir = committedPath
				cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
				out, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "git setup: "+string(out))
			}

			// Two independent checkouts of the exact same commit, baked
			// a few seconds apart, so any wall-clock-derived nondeterminism
			// in `bosh create-release` would show up as a checksum mismatch.
			pathA := filepath.Join(tmpRoot, "checkout-a")
			pathB := filepath.Join(tmpRoot, "checkout-b")
			Expect(os.CopyFS(pathA, os.DirFS(committedPath))).To(Succeed())
			time.Sleep(2 * time.Second)
			Expect(os.CopyFS(pathB, os.DirFS(committedPath))).To(Succeed())

			bakerA := NewBaker()
			bakerA.SetWriter(GinkgoWriter)
			Expect(bakerA.Bake(pathA, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})).To(Succeed())
			tarballA, err := bakerA.GetReleaseTarball()
			Expect(err).NotTo(HaveOccurred())

			bakerB := NewBaker()
			bakerB.SetWriter(GinkgoWriter)
			Expect(bakerB.Bake(pathB, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})).To(Succeed())
			tarballB, err := bakerB.GetReleaseTarball()
			Expect(err).NotTo(HaveOccurred())

			Expect(filepath.Base(tarballA)).To(Equal(filepath.Base(tarballB)),
				"same source commit should produce the same bosh release name+version")
			Expect(fileChecksum(tarballB)).To(Equal(fileChecksum(tarballA)),
				"same source commit should produce a byte-identical bosh release tarball across independent bakes")
		})
	})

	Context("release version across commits", func() {
		It("keeps the same version across an empty commit, but keeps real commit provenance", func() {
			// End-to-end sanity check of the intended design against the
			// real bosh CLI: the release version must be insensitive to
			// which commit a byte-identical tile was built from, while
			// release.MF still carries the true commit_hash for each build.
			// The version side of this held even before finalizeReleaseVersion
			// existed (the old pre-build hash never saw git-derived fields
			// either) -- the actual bug it fixes is that the version could
			// silently diverge from what bosh's own job/package fingerprints
			// produced; see TestFinalizeReleaseVersionChangesWhenBlobChecksumChanges
			// for the test that exercises that mechanism directly.
			tmpRoot, err := os.MkdirTemp("", "version-across-commits-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(tmpRoot) }()

			inputPath := filepath.Join(tmpRoot, "tile")
			Expect(os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))).To(Succeed())

			gitEnv := append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
			runGit := func(args ...string) {
				cmd := exec.Command("git", args...)
				cmd.Dir = inputPath
				cmd.Env = gitEnv
				out, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "git "+strings.Join(args, " ")+": "+string(out))
			}
			runGit("init")
			runGit("add", ".")
			runGit("commit", "-m", "initial commit")

			bakerA := NewBaker()
			bakerA.SetWriter(GinkgoWriter)
			Expect(bakerA.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})).To(Succeed())
			tarballA, err := bakerA.GetReleaseTarball()
			Expect(err).NotTo(HaveOccurred())
			manifestA, err := readReleaseManifest(tarballA)
			Expect(err).NotTo(HaveOccurred())

			// Same tree, new commit -- zero content change, but bosh will
			// stamp a different commit_hash.
			runGit("commit", "--allow-empty", "-m", "second commit, no content change")

			bakerB := NewBaker()
			bakerB.SetWriter(GinkgoWriter)
			Expect(bakerB.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})).To(Succeed())
			tarballB, err := bakerB.GetReleaseTarball()
			Expect(err).NotTo(HaveOccurred())
			manifestB, err := readReleaseManifest(tarballB)
			Expect(err).NotTo(HaveOccurred())

			Expect(bakerB.GetReleaseVersion()).To(Equal(bakerA.GetReleaseVersion()),
				"identical tile content must keep the same release version across an unrelated commit")

			Expect(manifestA.CommitHash).NotTo(BeEmpty())
			Expect(manifestB.CommitHash).NotTo(BeEmpty())
			Expect(manifestB.CommitHash).NotTo(Equal(manifestA.CommitHash),
				"commit_hash is real provenance and must still reflect the actual commit each release was built from")
		})

		It("changes the release version when the tile content actually changes", func() {
			tmpRoot, err := os.MkdirTemp("", "version-tracks-content-*")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = os.RemoveAll(tmpRoot) }()

			inputPath := filepath.Join(tmpRoot, "tile")
			Expect(os.CopyFS(inputPath, os.DirFS("testdata/sample-tile"))).To(Succeed())

			gitEnv := append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
			runGit := func(args ...string) {
				cmd := exec.Command("git", args...)
				cmd.Dir = inputPath
				cmd.Env = gitEnv
				out, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "git "+strings.Join(args, " ")+": "+string(out))
			}
			runGit("init")
			runGit("add", ".")
			runGit("commit", "-m", "initial commit")

			bakerA := NewBaker()
			bakerA.SetWriter(GinkgoWriter)
			Expect(bakerA.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})).To(Succeed())
			versionA := bakerA.GetReleaseVersion()

			// bundle.tar is what bosh add-blob packages into .boshrelease, so
			// changing it (unlike tile-only metadata such as icon.png) must
			// change the release fingerprint.
			bundlePath := filepath.Join(inputPath, "bundle.tar")
			existing, err := os.ReadFile(bundlePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(bundlePath, append(existing, 0x00), 0o644)).To(Succeed())
			runGit("add", ".")
			runGit("commit", "-m", "change packaged content")

			bakerB := NewBaker()
			bakerB.SetWriter(GinkgoWriter)
			Expect(bakerB.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, BakeOptions{})).To(Succeed())
			versionB := bakerB.GetReleaseVersion()

			Expect(versionB).NotTo(Equal(versionA),
				"changing packaged tile content must change the release version")
		})
	})
})
