package carvel

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"gopkg.in/yaml.v3"
)

func boshInstalled() bool {
	_, err := exec.LookPath("bosh")
	return err == nil
}

func kilnInstalled() bool {
	_, err := exec.LookPath("kiln")
	return err == nil
}

var _ = Describe("Carvel Baker", func() {
	Context("Bake", func() {
		When("the input directory contains k8s tile data", func() {
			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
			})
			var (
				inputPath, outputPath string
				subject               Baker
				err                   error
			)
			BeforeEach(func() {
				var err error
				inputPath, err = os.MkdirTemp("", "testinput-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				outputPath = path.Join(inputPath, ".ezbake")
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
					os.RemoveAll(filepath.Dir(inputPath))
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
})
