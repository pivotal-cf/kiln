//go:build ops_manifest

package planitest_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/pivotal-cf/kiln/pkg/planitest"
)

func Example() {
	tileFile, err := os.Open("acceptance/fixtures/fake-tile-metadata.yml")
	if err != nil {
		panic(err)
	}
	defer tileFile.Close()

	configFile := strings.NewReader(`---
network-properties:
  network:
    name: some-network
  other_availability_zones:
  - name: some-az
  singleton_availability_zone:
    name: some-az
product-properties: {}
`)

	_ = os.Setenv("RENDERER", "ops-manifest")
	product, err := planitest.NewProductService(planitest.ProductConfig{
		ConfigFile: configFile,
		TileFile:   tileFile,
	})
	if err != nil {
		panic(err)
	}

	manifest, err := product.RenderManifest(map[string]interface{}{
		".properties.required": "foo",
	})
	if err != nil {
		panic(err)
	}

	job, err := manifest.FindInstanceGroupJob("some-instance-group", "some-job")
	if err != nil {
		panic(err)
	}

	withDefault, err := job.Property("with_default")
	if err != nil {
		panic(err)
	}

	required, err := job.Property("required")
	if err != nil {
		panic(err)
	}

	fmt.Println(withDefault)
	fmt.Println(required)

	// Output:
	// some-default
	// foo
}
