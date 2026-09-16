package models_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/internal/carvel/models"
)

func TestPackageInstallUnmarshalDownwardAPIItems(t *testing.T) {
	g := NewWithT(t)

	yamlContent := `
name: tnz-ear-runtime-package-install
packageName: diego-rep.tanzu.vmware.com
packageVersion: 1.0.0
downwardAPIItems:
- name: group_versions
  kubernetesAPIs: {}
`
	var pi models.PackageInstall
	err := yaml.Unmarshal([]byte(yamlContent), &pi)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(pi.Name).To(Equal("tnz-ear-runtime-package-install"))
	g.Expect(pi.PackageName).To(Equal("diego-rep.tanzu.vmware.com"))
	g.Expect(pi.PackageVersion).To(Equal("1.0.0"))
	g.Expect(pi.DownwardAPIItems).To(HaveLen(1))
	g.Expect(pi.DownwardAPIItems[0].Name).To(Equal("group_versions"))
	g.Expect(pi.DownwardAPIItems[0].KubernetesAPIs).NotTo(BeNil())
}

func TestPackageInstallUnmarshalNoDownwardAPIItems(t *testing.T) {
	g := NewWithT(t)

	yamlContent := `
name: simple-pkg
packageName: simple.tanzu.vmware.com
packageVersion: 1.0.0
`
	var pi models.PackageInstall
	err := yaml.Unmarshal([]byte(yamlContent), &pi)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(pi.DownwardAPIItems).To(BeEmpty())
}
