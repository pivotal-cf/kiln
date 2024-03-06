package bake

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
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
	KilnVersion string `yaml:"kiln_version,omitempty" json:"kiln_version,omitempty"`

	// TileName might record the tile name used in baking because sometimes multiple tiles can be generated from the same tile source directory
	// An example of this is the two Tanzu Application Service tiles with different topologies listed on TanzuNetwork.
	TileName string `yaml:"tile_name,omitempty" json:"tile_name,omitempty"`

	// FileChecksum may be the SHA256 checksum of the baked tile.
	FileChecksum string `yaml:"file_checksum,omitempty" json:"file_checksum,omitempty"`

	// TileDirectory may be the directory containing tile source.
	TileDirectory string `yaml:"tile_directory,omitempty" json:"tile_directory,omitempty"`
}

// NewRecord parses build information from an OpsManger Product Template (aka metadata/metadata.yml)
func NewRecord(fileChecksum string, productTemplateBytes []byte) (Record, error) {
	var productTemplate struct {
		ProductVersion string               `yaml:"product_version"`
		KilnMetadata   builder.KilnMetadata `yaml:"kiln_metadata"`
	}

	err := yaml.Unmarshal(productTemplateBytes, &productTemplate)
	if err != nil {
		return Record{}, err
	}

	if productTemplate.KilnMetadata.MetadataGitSHA == "" {
		return Record{}, fmt.Errorf("failed to parse build information from product template: kiln_metadata.metadata_git_sha not found")
	}

	return Record{
		SourceRevision: productTemplate.KilnMetadata.MetadataGitSHA,
		Version:        productTemplate.ProductVersion,
		TileName:       productTemplate.KilnMetadata.TileName,
		FileChecksum:   fileChecksum,
	}, nil
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

func (record Record) Name() string {
	if record.TileName != "" {
		return path.Join(record.TileName, record.Version)
	}
	return record.Version
}

func (record Record) CompareVersion(o Record) int {
	bv, err := semver.NewVersion(record.Version)
	if err != nil {
		return strings.Compare(record.Version, o.Version)
	}
	ov, err := semver.NewVersion(o.Version)
	if err != nil {
		return strings.Compare(record.Version, o.Version)
	}
	return bv.Compare(ov)
}

func (record Record) CompareTileName(o Record) int {
	return strings.Compare(record.TileName, o.TileName)
}

func (record Record) IsDevBuild() bool {
	return record.SourceRevision == builder.DirtyWorktreeSHAValue
}

func (record Record) IsEquivalent(other Record, logger *log.Logger) bool {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	// if tile records differ on the following fields the tiles are still close enough
	record.KilnVersion = ""
	other.KilnVersion = ""
	record.TileDirectory = ""
	other.TileDirectory = ""
	record.TileName = ""
	other.TileName = ""

	if exp, got := record.Version, other.Version; exp != got {
		logger.Printf("tile versions are not the same: expected %q but got %q", exp, got)
	}

	if exp, got := record.SourceRevision, other.SourceRevision; exp != got {
		logger.Printf("tile source revisions are not the same: expected %q but got %q", exp, got)
	}

	if exp, got := record.FileChecksum, other.FileChecksum; exp != got {
		logger.Printf("tile file checksums are not the same: expected %q but got %q", exp, got)
	}

	return record == other
}

func (record Record) WriteFile(tileSourceDirectory string) error {
	if record.Version == "" {
		return fmt.Errorf("missing required version field")
	}
	if record.IsDevBuild() {
		return fmt.Errorf("will not write development builds to %s directory", RecordsDirectory)
	}
	if err := os.MkdirAll(filepath.Join(tileSourceDirectory, RecordsDirectory), 0o766); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	fileName := record.Version + ".json"
	if record.TileName != "" {
		fileName = record.TileName + "-" + fileName
	}
	outputFilepath := filepath.Join(tileSourceDirectory, RecordsDirectory, fileName)
	if _, err := os.Stat(outputFilepath); err == nil {
		return fmt.Errorf("tile bake record already exists for %s", record.Name())
	}
	return os.WriteFile(outputFilepath, buf, 0o644)
}

func (record Record) SetTileDirectory(tileSourceDirectory string) (Record, error) {
	absoluteTileSourceDirectory, err := filepath.Abs(tileSourceDirectory)
	if err != nil {
		return record, err
	}
	repoRoot, err := repositoryRoot(tileSourceDirectory)
	if err != nil {
		return record, err
	}
	repoRoot += string(filepath.Separator)
	absoluteTileSourceDirectory += string(filepath.Separator)
	if !strings.HasPrefix(absoluteTileSourceDirectory, repoRoot) {
		return record, fmt.Errorf("expected tile directory %q to either be or be a child of the repository root directory %q", absoluteTileSourceDirectory, repoRoot)
	}
	relativeTilePath := strings.TrimPrefix(absoluteTileSourceDirectory, repoRoot)
	record.TileDirectory = filepath.ToSlash(filepath.Clean(relativeTilePath))
	return record, nil
}

func repositoryRoot(dir string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get HEAD revision hash: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
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
