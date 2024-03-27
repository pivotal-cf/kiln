package bake

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func Test_shatterFromFS(t *testing.T) {
	t.Run("missing product template", func(t *testing.T) {
		zip := os.DirFS(t.TempDir())
		output := t.TempDir()
		err := newFromFS(output, zip, cargo.Kilnfile{})
		require.ErrorContains(t, err, "metadata file not found")
	})
}

func Test_shatterProductTemplate(t *testing.T) {
	t.Run("when the product template is valid", func(t *testing.T) {
		const productTemplateYAML = `}:`
		output := t.TempDir()
		_, err := newFromProductTemplate(output, []byte(productTemplateYAML))
		require.ErrorContains(t, err, "failed to parse product template")
	})
}

func Test_writeIconPNG(t *testing.T) {
	t.Run("when the field is valid base64", func(t *testing.T) {
		const productTemplateYAML =
		/* language=yaml */ `---
icon_image: cmVsYXRpbmcgdG8gb3Igb2YgdGhlIG5hdHVyZSBvZiBhbiBpY29u
`
		output := t.TempDir()
		productTemplate := parseProductTemplateNode(t, productTemplateYAML)
		err := writeIconPNG(output, productTemplate)
		require.NoError(t, err)

		expOutput := filepath.Join(output, DefaultFilepathIconImage)
		require.FileExists(t, expOutput)
		buf, err := os.ReadFile(expOutput)
		require.NoError(t, err)
		require.Equal(t, "relating to or of the nature of an icon", string(buf),
			"it gets written to the file")
	})

	t.Run("missing icon", func(t *testing.T) {
		const productTemplateYAML =
		/* language=yaml */ `---
ping: pong
`
		output := t.TempDir()
		productTemplate := parseProductTemplateNode(t, productTemplateYAML)
		err := writeIconPNG(output, productTemplate)
		require.ErrorContains(t, err, "icon_image not found in product template")
	})

	t.Run("not base64", func(t *testing.T) {
		const productTemplateYAML =
		/* language=yaml */ `---
icon_image: $
`
		output := t.TempDir()
		productTemplate := parseProductTemplateNode(t, productTemplateYAML)
		err := writeIconPNG(output, productTemplate)
		require.ErrorContains(t, err, "failed to decode icon_image")
	})
}

func parseProductTemplateNode(t *testing.T, productTemplateYAML string) *yaml.Node {
	t.Helper()
	var productTemplate yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(productTemplateYAML), &productTemplate))
	return &productTemplate
}
