package scenario

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/internal/proofing"
	"os"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

// aTileIsCreated asserts the output tile exists
func aTileIsCreated(ctx context.Context) error {
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	_, err = os.Stat(tilePath)
	return err
}

// theTileContains checks that the filePaths exist in the tile
func theTileContains(ctx context.Context, table *godog.Table) error {
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	tile, err := zip.OpenReader(tilePath)
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			_, err := tile.Open(cell.Value)
			if err != nil {
				return fmt.Errorf("tile did not contain file %s", cell.Value)
			}
		}
	}
	return nil
}

func theTileOnlyContainsCompiledReleases(ctx context.Context) error {
	tilePath, err := defaultFilePathForTile(ctx)
	if err != nil {
		return err
	}
	metadataBuf, err := tile.ReadMetadataFromFile(tilePath)
	if err != nil {
		return err
	}

	var metadata struct {
		Releases []proofing.Release `yaml:"releases"`
	}
	err = yaml.Unmarshal(metadataBuf, &metadata)
	if err != nil {
		return err
	}

	for _, release := range metadata.Releases {
		helloReleaseTarball := bytes.NewBuffer(nil)
		_, err := tile.ReadReleaseFromFile(tilePath, release.Name, release.Version, helloReleaseTarball)
		if err != nil {
			return err
		}
		manifestBuf, err := component.ReadReleaseManifest(helloReleaseTarball)
		if err != nil {
			return err
		}
		err = validateAllPackagesAreCompiled(manifestBuf, release)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateAllPackagesAreCompiled asserts that if any package is listed under "packages", it is not complied.
// It does not ensure "compiled_packages" is populated.
func validateAllPackagesAreCompiled(manifestBuf []byte, release proofing.Release) error {
	var manifest struct {
		Packages []struct {
			Name string `yaml:"name"`
		} `yaml:"packages"`
	}
	err := yaml.Unmarshal(manifestBuf, &manifest)
	if err != nil {
		return err
	}
	if len(manifest.Packages) > 0 {
		sb := new(strings.Builder)
		sb.WriteString(fmt.Sprintf("release %s/%s contains un-compiled packages: ", release.Name, release.Version))
		for i, p := range manifest.Packages {
			sb.WriteString(p.Name)
			if i < len(manifest.Packages)-1 {
				sb.WriteString(", ")
			}
		}
		return errors.New(sb.String())
	}
	return nil
}
