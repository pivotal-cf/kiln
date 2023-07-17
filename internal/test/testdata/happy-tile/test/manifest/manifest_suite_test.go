package manifest_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/planitest"
)

type Job struct {
	InstanceGroup string
	Name          string
}

func TestManifestGeneration(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manifest Generation Suite")
}

var (
	product       *planitest.ProductService
	productConfig planitest.ProductConfig
	metadataFile  string
	productName   string
	configFile    *os.File
)

func kilnBake(args ...string) (string, error) {
	cmd := exec.Command("kiln", append([]string{"bake"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	cmd.Dir = "../../"
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return stdout.String(), nil
}

var _ = SynchronizedBeforeSuite(func() []byte {
	os.Setenv("RENDERER", "ops-manifest")
	productToBuild := os.Getenv("PRODUCT")
	if productToBuild == "" {
		productToBuild = "srt"
	}

	fmt.Printf("Testing product: %s", productToBuild)

	output, err := kilnBake(
		"--variable=metadata-git-sha=develop",
		"--variables-file=variables/srt.yml",
		"--stub-releases",
		"--metadata-only")
	if err != nil {
		Expect(err).NotTo(HaveOccurred())
	}

	return []byte(output)
}, func(metadataContents []byte) {
	metadataFile = string(metadataContents)
	productName = os.Getenv("PRODUCT")
	if productName == "" {
		productName = "srt"
	}

	var err error

	configFile, err = os.Open("config.json")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	_ = configFile.Close()
})

var _ = BeforeEach(func() {
	var err error
	productConfig = planitest.ProductConfig{
		ConfigFile: configFile,
		TileFile:   strings.NewReader(metadataFile),
	}
	product, err = planitest.NewProductService(productConfig)
	Expect(err).NotTo(HaveOccurred())
})
