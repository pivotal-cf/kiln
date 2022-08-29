package cargo

import (
	"testing"

	. "github.com/onsi/gomega"

	"gopkg.in/yaml.v2"
)

func TestComponentLock_yaml_marshal_order(t *testing.T) {
	const validComponentLockYaml = `name: fake-component-name
sha1: fake-component-sha1
version: fake-version
remote_source: fake-source
remote_path: fake/path/to/fake-component-name
`
	damnit := NewWithT(t)

	cl, err := yaml.Marshal(ComponentLock{
		Name:         "fake-component-name",
		Version:      "fake-version",
		SHA1:         "fake-component-sha1",
		RemoteSource: "fake-source",
		RemotePath:   "fake/path/to/fake-component-name",
	})

	damnit.Expect(err).NotTo(HaveOccurred())
	damnit.Expect(string(cl)).To(Equal(validComponentLockYaml))
}

func TestStemcell_ProductSlug(t *testing.T) {
	tests := []struct {
		name      string
		stemcell  Stemcell
		matchErr  OmegaMatcher
		matchSlug OmegaMatcher
	}{
		{name: "empty", stemcell: Stemcell{}, matchErr: MatchError(ContainSubstring("not set"))},
		{name: "xenial", stemcell: Stemcell{
			OS: "ubuntu-xenial",
		}, matchErr: Not(HaveOccurred()), matchSlug: Equal("stemcells-ubuntu-xenial")},
		{name: "jammy", stemcell: Stemcell{
			OS: "ubuntu-jammy",
		}, matchErr: Not(HaveOccurred()), matchSlug: Equal("stemcells-ubuntu-jammy")},
		{name: "windows", stemcell: Stemcell{
			OS: "windows2019",
		}, matchErr: Not(HaveOccurred()), matchSlug: Equal("stemcells-windows-server")},
		{name: "uses slug field over fallback", stemcell: Stemcell{
			OS:   "ubuntu-xenial",
			Slug: "banana",
		}, matchErr: Not(HaveOccurred()), matchSlug: Equal("banana")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, err := tt.stemcell.ProductSlug()

			// then
			please := NewWithT(t)
			if tt.matchErr != nil {
				please.Expect(err).To(tt.matchErr)
			}
			if tt.matchSlug != nil {
				please.Expect(got).To(tt.matchSlug)
			}
		})
	}
}
