package bake

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Masterminds/semver/v3"

	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

// RecordsDirectory should be a sibling to Kilnfile or base.yml
const RecordsDirectory = "bake_records"

// Record is created by the function
type Record struct {
	// SourceRevision is the commit checked out when the build was run
	SourceRevision string `yaml:"source_revision" json:"source_revision"`

	// Version is the tile version used for product template field `product_version`
	Version string `yaml:"version" json:"version"`

	// KilnVersion is the Kiln CLI version
	KilnVersion string `yaml:"kiln_version" json:"kiln_version"`

	// TileName might record the tile name used in baking because sometimes multiple tiles can be generated from the same tile source directory
	// An example of this is the two Tanzu Application Service tiles with different topologies listed on TanzuNetwork.
	TileName string `yaml:"tile_name,omitempty" json:"tile_name,omitempty"`

	// FileChecksum is the SHA256 checksum of the baked tile.
	FileChecksum string `yaml:"file_checksum,omitempty" json:"file_checksum,omitempty"`
}

// NewRecord parses build information from an OpsManger Product Template (aka metadata/metadata.yml)
func NewRecord(fileChecksum string, productTemplateBytes []byte) (Record, error) {
	var productTemplate struct {
		ProductVersion string               `yaml:"product_version"`
		KilnMetadata   builder.KilnMetadata `yaml:"kiln_metadata"`
	}

	err := yaml.Unmarshal(productTemplateBytes, &productTemplate)

	if productTemplate.KilnMetadata.KilnVersion == "" {
		return Record{}, fmt.Errorf("failed to parse build information from product template: kiln_metadata.kiln_version not found")
	}

	return Record{
		SourceRevision: productTemplate.KilnMetadata.MetadataGitSHA,
		Version:        productTemplate.ProductVersion,
		KilnVersion:    productTemplate.KilnMetadata.KilnVersion,
		TileName:       productTemplate.KilnMetadata.TileName,
		FileChecksum:   fileChecksum,
	}, err
}

// NewRecordFromFile parses the product template and pulls the kiln metadata out.
// The SHA256 sum is also calculated for the file.
func NewRecordFromFile(tileFilepath string) (Record, error) {
	metadata, err := tile.ReadMetadataFromFile(tileFilepath)
	if err != nil {
		return Record{}, err
	}
	checksum, err := fileChecksum(tileFilepath)
	if err != nil {
		return Record{}, err
	}
	return NewRecord(checksum, metadata)
}

func fileChecksum(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreError(f)
	sum := sha256.New()
	_, err = io.Copy(sum, f)
	return hex.EncodeToString(sum.Sum(nil)), err
}

func ReadRecords(dir fs.FS) ([]Record, error) {
	infos, err := fs.ReadDir(dir, RecordsDirectory)
	if err != nil {
		return nil, err
	}
	builds := make([]Record, 0, len(infos))
	for _, info := range infos {
		buf, err := fs.ReadFile(dir, path.Join(RecordsDirectory, info.Name()))
		if err != nil {
			return nil, err
		}
		var build Record
		if err := json.Unmarshal(buf, &build); err != nil {
			return nil, err
		}
		builds = append(builds, build)
	}
	slices.SortFunc(builds, compareMultiple(Record.CompareVersion, Record.CompareTileName))
	return builds, nil
}

func (b Record) Name() string {
	if b.TileName != "" {
		return path.Join(b.TileName, b.Version)
	}
	return b.Version
}

func (b Record) CompareVersion(o Record) int {
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

func (b Record) CompareTileName(o Record) int {
	return strings.Compare(b.TileName, o.TileName)
}

func (b Record) IsDevBuild() bool {
	return b.SourceRevision == builder.DirtyWorktreeSHAValue
}

func (b Record) WriteFile(tileSourceDirectory string) error {
	if b.Version == "" {
		return fmt.Errorf("missing required version field")
	}
	if b.IsDevBuild() {
		return fmt.Errorf("will not write development builds to %s directory", RecordsDirectory)
	}
	if err := os.MkdirAll(filepath.Join(tileSourceDirectory, RecordsDirectory), 0o766); err != nil {
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
	outputFilepath := filepath.Join(tileSourceDirectory, RecordsDirectory, fileName)
	if _, err := os.Stat(outputFilepath); err == nil {
		return fmt.Errorf("tile bake record already exists for %s", b.Name())
	}
	return os.WriteFile(outputFilepath, buf, 0o644)
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

func closeAndIgnoreError(closer io.Closer) {
	_ = closer.Close()
}
