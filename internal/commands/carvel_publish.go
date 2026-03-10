package commands

import (
	"crypto/sha256"
	"encoding/hex"
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

type CarvelPublish struct {
	outLogger   *log.Logger
	errLogger   *log.Logger
	KilnVersion string
	Options     CarvelPublishOptions
}

type CarvelPublishOptions struct {
	SourceDirectory string `short:"s" long:"source-directory" description:"path to the Carvel tile source directory (defaults to current directory)"`
	OutputFile      string `short:"o" long:"output-file"      description:"path to where the tile will be output" required:"true"`
	Version         string `          long:"version"           description:"tile version for the final release"`
	Lockfile        string `short:"l" long:"lockfile"          description:"path to Kilnfile.lock for using a cached BOSH release"`
	Verbose         bool   `short:"v" long:"verbose"           description:"enable verbose output"`
	IsFinal         bool   `          long:"final"             description:"create a bake record for this build"`
}

func NewCarvelPublish(outLogger, errLogger *log.Logger) CarvelPublish {
	return CarvelPublish{
		outLogger: outLogger,
		errLogger: errLogger,
	}
}

func (c CarvelPublish) Execute(args []string) error {
	_, err := jhanda.Parse(&c.Options, args)
	if err != nil {
		return err
	}

	sourcePath := c.Options.SourceDirectory
	if sourcePath == "" {
		sourcePath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		sourcePath, err = filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to resolve source directory: %w", err)
		}
	}

	targetPath, err := filepath.Abs(c.Options.OutputFile)
	if err != nil {
		return fmt.Errorf("failed to resolve output file path: %w", err)
	}

	b := carvel.NewBaker()
	if c.Options.Verbose {
		b.SetWriter(os.Stdout)
	}

	if c.Options.Lockfile != "" {
		lockfilePath, err := filepath.Abs(c.Options.Lockfile)
		if err != nil {
			return fmt.Errorf("failed to resolve lockfile path: %w", err)
		}
		c.outLogger.Printf("Publishing Carvel tile from %s using lockfile %s", sourcePath, lockfilePath)
		err = b.BakeFromLockfile(sourcePath, lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to prepare Carvel tile from lockfile: %w", err)
		}
	} else {
		c.outLogger.Printf("Publishing Carvel tile from %s", sourcePath)
		err = b.Bake(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to prepare Carvel tile: %w", err)
		}
	}

	ver, err := b.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get tile version: %w", err)
	}
	if c.Options.Version != "" {
		ver = c.Options.Version
	}

	err = b.KilnBake(targetPath)
	if err != nil {
		return fmt.Errorf("failed to bake tile: %w", err)
	}

	c.outLogger.Printf("Baked %s version %s to %s", b.GetName(), ver, targetPath)

	if c.Options.IsFinal {
		// Resolve symlinks so git's toplevel and our absolute path match (macOS /var -> /private/var)
		resolvedSourcePath, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			resolvedSourcePath = sourcePath
		}

		sha, err := builder.GitMetadataSHA(resolvedSourcePath, false)
		if err != nil {
			return fmt.Errorf("failed to get git SHA: %w", err)
		}

		checksum, err := tileFileChecksum(targetPath)
		if err != nil {
			return fmt.Errorf("failed to checksum tile: %w", err)
		}

		record := bake.Record{
			SourceRevision: sha,
			Version:        ver,
			KilnVersion:    c.KilnVersion,
			FileChecksum:   checksum,
		}

		record, err = record.SetTileDirectory(resolvedSourcePath)
		if err != nil {
			return fmt.Errorf("failed to set tile directory on bake record: %w", err)
		}

		err = record.WriteFile(resolvedSourcePath)
		if err != nil {
			return fmt.Errorf("failed to write bake record: %w", err)
		}

		c.outLogger.Printf("Wrote bake record for version %s", ver)
	}

	return nil
}

func (c CarvelPublish) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Publishes a Carvel/Kubernetes tile as a .pivotal file. When --final is specified, creates a bake record that can be used with 'kiln carvel rebake' for reproducible builds.",
		ShortDescription: "publishes a Carvel/Kubernetes tile",
		Flags:            c.Options,
	}
}

func tileFileChecksum(path string) (string, error) {
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
