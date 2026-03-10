package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/pkg/bake"
)

type CarvelReBake struct {
	outLogger *log.Logger
	errLogger *log.Logger
	Options   CarvelReBakeOptions
}

type CarvelReBakeOptions struct {
	OutputFile string `short:"o" long:"output-file" description:"path to where the tile will be output" required:"true"`
	Verbose    bool   `short:"v" long:"verbose"     description:"enable verbose output"`
}

func NewCarvelReBake(outLogger, errLogger *log.Logger) CarvelReBake {
	return CarvelReBake{
		outLogger: outLogger,
		errLogger: errLogger,
	}
}

func (c CarvelReBake) Execute(args []string) error {
	remaining, err := jhanda.Parse(&c.Options, args)
	if err != nil {
		return err
	}
	if len(remaining) != 1 {
		return fmt.Errorf("exactly one bake record argument is required, got %d", len(remaining))
	}

	recordPath := remaining[0]
	recordBuf, err := os.ReadFile(recordPath)
	if err != nil {
		return fmt.Errorf("failed to read bake record file: %w", err)
	}

	var record bake.Record
	if err := json.Unmarshal(recordBuf, &record); err != nil {
		return fmt.Errorf("failed to parse bake record: %w", err)
	}

	tileDir := filepath.FromSlash(record.TileDirectory)
	if tileDir == "" {
		tileDir = "."
	}

	sourcePath, err := filepath.Abs(tileDir)
	if err != nil {
		return fmt.Errorf("failed to resolve tile directory: %w", err)
	}

	workingDirectorySHA, err := builder.GitMetadataSHA(sourcePath, false)
	if err != nil {
		return err
	}

	if got, exp := workingDirectorySHA, record.SourceRevision; got != exp {
		return fmt.Errorf("expected worktree at source revision %s but current HEAD is %s", exp, got)
	}

	targetPath, err := filepath.Abs(c.Options.OutputFile)
	if err != nil {
		return fmt.Errorf("failed to resolve output file path: %w", err)
	}

	b := carvel.NewBaker()
	if c.Options.Verbose {
		b.SetWriter(os.Stdout)
	}

	lockfilePath := filepath.Join(sourcePath, "Kilnfile.lock")
	if _, statErr := os.Stat(lockfilePath); statErr == nil {
		c.outLogger.Printf("Re-baking Carvel tile from %s using lockfile", sourcePath)
		err = b.BakeFromLockfile(sourcePath, lockfilePath)
	} else {
		c.outLogger.Printf("Re-baking Carvel tile from %s", sourcePath)
		err = b.Bake(sourcePath)
	}
	if err != nil {
		return fmt.Errorf("failed to prepare Carvel tile: %w", err)
	}

	err = b.KilnBake(targetPath)
	if err != nil {
		return fmt.Errorf("failed to bake tile: %w", err)
	}

	checksum, err := rebakeFileChecksum(targetPath)
	if err != nil {
		return fmt.Errorf("failed to checksum tile: %w", err)
	}

	if record.FileChecksum != "" && record.FileChecksum != checksum {
		return fmt.Errorf("tile checksum mismatch: record has %s, rebake produced %s", record.FileChecksum, checksum)
	}

	ver, _ := b.GetVersion()
	c.outLogger.Printf("Re-baked %s version %s to %s", b.GetName(), ver, targetPath)

	return nil
}

func (c CarvelReBake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Re-bakes a Carvel tile from a bake record for reproducible builds. The repository must be checked out at the source_revision specified in the bake record.",
		ShortDescription: "re-bakes a Carvel tile from a bake record",
		Flags:            c.Options,
	}
}

func rebakeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
