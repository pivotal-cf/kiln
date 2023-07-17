package stability

import (
	"bytes"
	_ "embed"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gopkg.in/yaml.v2"
)

func TestFetcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stability Suite")
}

var _ = Describe("PropertyBlueprints", func() {
	Context("hello-tile", func() {
		It("doesn't have breaking changes", func() {
			helloTilePatchMetadata, err := kilnBake(istBakeArgs()...)
			Expect(err).To(BeNil())
			Expect(helloTilePatchMetadata.Name).To(Equal("example"))
		})
	})
})

func istBakeArgs() []string {
	return []string{
		"--variables-file=variables/srt.yml",
		"--variable=metadata-git-sha=develop",
		"--stub-releases",
		"--metadata-only",
	}
}

type metadata struct {
	Name string `yaml:"name"`
}

func kilnBake(args ...string) (metadata, error) {
	cmd := exec.Command("kiln", append([]string{"bake"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = "../../"
	err := cmd.Run()
	if err != nil {
		return metadata{}, err
	}
	var metadatas metadata
	err = yaml.Unmarshal(stdout.Bytes(), &metadatas)
	if err != nil {
		return metadata{}, err
	}
	return metadatas, nil
}
