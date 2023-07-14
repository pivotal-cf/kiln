package upgrade_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"

	"github.com/pivotal-cf/kiln/pkg/proofing"
	"github.com/pivotal-cf/kiln/pkg/proofing/upgrade"
)

func TestListBreakingChanges(t *testing.T) {
	t.Run("product_version", func(t *testing.T) {
		t.Run("stable has product_version patch number zero", func(t *testing.T) {
			// Happy path
			initialMetadata := proofing.ProductTemplate{ProductVersion: "3.0.0"}
			patchMetadata := proofing.ProductTemplate{ProductVersion: "3.0.1"}

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 0)
		})

		t.Run("stable has non-zero product_version patch number", func(t *testing.T) {
			initialMetadata := proofing.ProductTemplate{ProductVersion: "3.0.1"}
			patchMetadata := proofing.ProductTemplate{ProductVersion: "3.0.99"}

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
		})

		t.Run("both have the same product_version", func(t *testing.T) {
			initialMetadata := proofing.ProductTemplate{ProductVersion: "3.0.0"}
			patchMetadata := proofing.ProductTemplate{ProductVersion: "3.0.0"}

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 0)
		})

		t.Run("patch product_version is less than stable product_version", func(t *testing.T) {
			initialMetadata := proofing.ProductTemplate{ProductVersion: "3.1.0"}
			patchMetadata := proofing.ProductTemplate{ProductVersion: "3.0.0"}

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
		})

		t.Run("invalid initial", func(t *testing.T) {
			initialMetadata := proofing.ProductTemplate{ProductVersion: "bad version"}
			patchMetadata := proofing.ProductTemplate{ProductVersion: "3.0.0"}

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
		})
		t.Run("invalid patch", func(t *testing.T) {
			initialMetadata := proofing.ProductTemplate{ProductVersion: "3.0.0"}
			patchMetadata := proofing.ProductTemplate{ProductVersion: "bad version"}

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
		})
	})

	t.Run("contrived scenarios", func(t *testing.T) {
		t.Run("product name changed", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.ErrorContains(t, breakingChanges[0], `breaking change tile names are not the same`)
		})

		// property_blueprints
		t.Run("added configurable property without default", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for property with name "added_property": added configurable property without default`)
		})
		t.Run("added configurable property with default", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Empty(t, breakingChanges)
		})
		t.Run("added non configurable property", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Empty(t, breakingChanges)
		})
		t.Run("changed configurable property name", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for property with name "existing_property": removed or renamed configurable property`)
		})
		t.Run("changed configurable property to not be configurable", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for property with name "existing_property": changed configurable property to not be configurable`)
		})
		t.Run("changed configurable property type", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for property with name "existing_property": changed configurable property type: type changed from "integer" to "port"`)
		})
		t.Run("removed configurable property", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for property with name "useless_property": removed or renamed configurable property`)
		})
		t.Run("removed configurable property default", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for property with name "existing_property": removed configurable property default`)
		})
		t.Run("removed errand", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for errand with name "smoke_tests": removed`)
		})
		t.Run("removed configurable instance group", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for configurable instance group with name "ha_proxy": removed`)
		})
		t.Run("instance group configurability changed to false", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 1)
			assert.EqualError(t, breakingChanges[0], `breaking change for configurable instance group with name "ha_proxy": configurable changed to false`)
		})
		t.Run("instance group constraints added", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Empty(t, breakingChanges)
		})
		t.Run("instance group constraints tightened", func(t *testing.T) {
			initialMetadata, patchMetadata := loadMetadataProperties(t)

			breakingChanges := upgrade.ListBreakingChanges(initialMetadata, patchMetadata)

			assert.Len(t, breakingChanges, 2)
			assert.EqualError(t, breakingChanges[0], `breaking change for instance definition constraint with name "uaa": increased min constraint`)
			assert.EqualError(t, breakingChanges[1], `breaking change for instance definition constraint with name "ha_proxy": reduced max constraint`)
		})
	})
}

func loadMetadataProperties(t *testing.T) (initial, patch proofing.ProductTemplate) {
	t.Helper()
	readYAMLFile(t, filepath.Join("testdata", "breaking_changes", path.Base(t.Name()), "initial.yml"), &initial)
	readYAMLFile(t, filepath.Join("testdata", "breaking_changes", path.Base(t.Name()), "patch.yml"), &patch)
	return
}

func readYAMLFile(t *testing.T, filePath string, data any) {
	iBuf, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	err = yaml.Unmarshal(iBuf, data)
	if err != nil {
		t.Fatal(err)
	}
}
