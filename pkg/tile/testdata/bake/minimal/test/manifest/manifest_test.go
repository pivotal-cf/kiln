package manifest_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/pivotal-cf/kiln/pkg/planitest"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

func TestManifest(t *testing.T) {
	metadata := bytes.NewBuffer(nil)

	err := tile.Metadata(metadata, os.DirFS("../../"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	configContent, err := json.Marshal(map[string]any{
		"network-properties": struct{}{},
		"product-properties": map[string]map[string]string{
			".properties.port": {
				"value": "8443",
			},
		},
		"resources": struct{}{},
	})
	if err != nil {
		t.Fatal(err)
	}

	productConfig := planitest.ProductConfig{
		ConfigFile: bytes.NewReader(configContent),
		TileFile:   bytes.NewReader(metadata.Bytes()),
	}
	product, err := planitest.NewProductService(productConfig)
	if err != nil {
		t.Fatal(err)
	}

	manifestYAML, err := product.RenderManifest(nil)
	if err != nil {
		t.Fatal(err)
	}

	helloServerInstanceGroup, err := manifestYAML.FindInstanceGroupJob("hello-server", "hello-server")
	if err != nil {
		t.Fatal(err)
	}
	expectComparableProperty[int](t, helloServerInstanceGroup, "port", 8443)
}

func expectComparableProperty[T comparable](t *testing.T, m planitest.Manifest, name string, expected T) {
	portValue, err := m.Property(name)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := portValue.(T)
	if !ok {
		var zero T
		t.Errorf("expected %s to be an %T got %T", name, zero, portValue)
	}
	if value != expected {
		t.Errorf("incorrect %s value %v expected %v", name, value, expected)
	}
}
