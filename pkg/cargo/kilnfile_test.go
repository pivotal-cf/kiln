package cargo

import (
	Ω "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	"testing"
)

func TestComponentLock_yaml_marshal_order(t *testing.T) {
	const validComponentLockYaml = `name: fake-component-name
sha1: fake-component-sha1
version: fake-version
remote_source: fake-source
remote_path: fake/path/to/fake-component-name
`
	damnit := Ω.NewWithT(t)

	cl, err := yaml.Marshal(ComponentLock{
		Name:         "fake-component-name",
		Version:      "fake-version",
		SHA1:         "fake-component-sha1",
		RemoteSource: "fake-source",
		RemotePath:   "fake/path/to/fake-component-name",
	})

	damnit.Expect(err).NotTo(Ω.HaveOccurred())
	damnit.Expect(string(cl)).To(Ω.Equal(validComponentLockYaml))
}

// TODO: why don't we have a custom Marshal() implementation?  How do we not write os and stemcell to our lockfile?
//func TestComponentLock_yaml_marshal_fields(t *testing.T) {
//	const validComponentLockInput = `name: fake-component-name
//sha1: fake-component-sha1
//version: fake-version
//remote_source: fake-source
//remote_path: fake/path/to/fake-component-name
//os: fake-stemcell-os
//stemcell_version: fake-stemcell-version
//`
//	const validComponentLockOutput = `name: fake-component-name
//sha1: fake-component-sha1
//version: fake-version
//remote_source: fake-source
//remote_path: fake/path/to/fake-component-name
//`
//	damnit := Ω.NewWithT(t)
//
//	cl := ComponentLock{}
//	err := yaml.Unmarshal([]byte(validComponentLockInput), &cl)
//	damnit.Expect(err).NotTo(Ω.HaveOccurred())
//
//	myCl, err := yaml.Marshal(cl)
//	damnit.Expect(string(myCl)).To(Ω.Equal(validComponentLockOutput))
//}
