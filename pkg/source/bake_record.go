package source

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pivotal-cf/kiln/internal/builder"

	"github.com/Masterminds/semver/v3"
)

// BakeRecordsDirectory should be a sibling to Kilnfile or base.yml
const BakeRecordsDirectory = "bake_records"

// BakeRecord is created by the function
type BakeRecord struct {
	// SourceRevision is the commit checked out when the build was run
	SourceRevision string `yaml:"source_revision" json:"source_revision"`

	// Version is the tile version used for product template field `product_version`
	Version string `yaml:"version" json:"version"`

	// KilnVersion is the Kiln CLI version
	KilnVersion string `yaml:"kiln_version" json:"kiln_version"`

	// TileName might record the tile name used in baking because sometimes multiple tiles can be generated from the same tile source directory
	// An example of this is the two Tanzu Application Service tiles with different topologies listed on TanzuNetwork.
	TileName string `yaml:"tile_name,omitempty" json:"tile_name,omitempty"`
}

// NewBakeRecord parses build information from an OpsManger Product Template (aka metadata/metadata.yml)
func NewBakeRecord(productTemplateBytes []byte) (BakeRecord, error) {
	var productTemplate struct {
		ProductVersion string               `yaml:"product_version"`
		KilnMetadata   builder.KilnMetadata `yaml:"kiln_metadata"`
	}

	err := yaml.Unmarshal(productTemplateBytes, &productTemplate)

	if productTemplate.KilnMetadata.KilnVersion == "" {
		return BakeRecord{}, fmt.Errorf("failed to parse build information from product template: kiln_metadata.kiln_version not found")
	}

	return BakeRecord{
		SourceRevision: productTemplate.KilnMetadata.MetadataGitSHA,
		Version:        productTemplate.ProductVersion,
		KilnVersion:    productTemplate.KilnMetadata.KilnVersion,
		TileName:       productTemplate.KilnMetadata.TileName,
	}, err
}

func (b BakeRecord) Name() string {
	if b.TileName != "" {
		return path.Join(b.TileName, b.Version)
	}
	return b.Version
}

func (b BakeRecord) CompareVersion(o BakeRecord) int {
	bv, err := semver.NewVersion(b.Version)
	if err != nil {
		return strings.Compare(b.Version, o.Version)
	}
	ov, err := semver.NewVersion(o.Version)
	if err != nil {
		return strings.Compare(b.Version, o.Version)
	}
	return bv.Compare(ov)
}

func (b BakeRecord) CompareTileName(o BakeRecord) int {
	return strings.Compare(b.TileName, o.TileName)
}

func (b BakeRecord) IsDevBuild() bool {
	return b.SourceRevision == builder.DirtyWorktreeSHAValue
}

func (b BakeRecord) WriteFile(tileSourceDirectory string) error {
	if b.Version == "" {
		return fmt.Errorf("missing required version field")
	}
	if b.IsDevBuild() {
		return fmt.Errorf("will not write development builds to %s directory", BakeRecordsDirectory)
	}
	if err := os.MkdirAll(filepath.Join(tileSourceDirectory, BakeRecordsDirectory), 0766); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	fileName := b.Version + ".json"
	if b.TileName != "" {
		fileName = b.TileName + "-" + fileName
	}
	outputFilepath := filepath.Join(tileSourceDirectory, BakeRecordsDirectory, fileName)
	if _, err := os.Stat(outputFilepath); err == nil {
		return fmt.Errorf("tile bake record already exists for %s", b.Name())
	}
	return os.WriteFile(outputFilepath, buf, 0644)
}

func ReadBakeRecords(dir fs.FS) ([]BakeRecord, error) {
	infos, err := fs.ReadDir(dir, BakeRecordsDirectory)
	if err != nil {
		return nil, err
	}
	builds := make([]BakeRecord, 0, len(infos))
	for _, info := range infos {
		buf, err := fs.ReadFile(dir, path.Join(BakeRecordsDirectory, info.Name()))
		if err != nil {
			return nil, err
		}
		var build BakeRecord
		if err := json.Unmarshal(buf, &build); err != nil {
			return nil, err
		}
		builds = append(builds, build)
	}
	slices.SortFunc(builds, compareMultiple(BakeRecord.CompareVersion, BakeRecord.CompareTileName))
	return builds, nil
}

func compareMultiple[T any](cmp ...func(a, b T) int) func(a, b T) int {
	return func(a, b T) int {
		var result int
		for _, c := range cmp {
			result = c(a, b)
			if result != 0 {
				break
			}
		}
		return result
	}
}
