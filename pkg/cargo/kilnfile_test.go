package cargo

import (
	"testing"

	Ω "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
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
