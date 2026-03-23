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
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/bake"
)

type CarvelPublish struct {
	outLogger   *log.Logger
	errLogger   *log.Logger
	KilnVersion string
	Options     CarvelPublishOptions
}

type CarvelPublishOptions struct {
	flags.Standard
	SourceDirectory string `short:"s" long:"source-directory" description:"path to the Carvel tile source directory (defaults to current directory)"`
	OutputFile      string `short:"o" long:"output-file"      description:"path to where the tile will be output" required:"true"`
	Version         string `          long:"version"           description:"tile version for the final release"`
	IsFinal         bool   `          long:"final"             description:"create a bake record for this build"`
	Verbose         bool   `short:"v" long:"verbose"           description:"enable verbose output"`
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

	sourcePath, err := resolveSourcePath(c.Options.SourceDirectory)
	if err != nil {
		return err
	}

	targetPath, err := filepath.Abs(c.Options.OutputFile)
	if err != nil {
		return fmt.Errorf("failed to resolve output file path: %w", err)
	}

	kilnfilePath := resolveKilnfilePath(c.Options.Kilnfile, sourcePath)

	if _, statErr := os.Stat(kilnfilePath); statErr != nil {
		return fmt.Errorf("could not find Kilnfile at %s: run 'kiln carvel upload' first to create the BOSH release, Kilnfile, and Kilnfile.lock", kilnfilePath)
	}

	lockfilePath := kilnfilePath + ".lock"
	if _, statErr := os.Stat(lockfilePath); statErr != nil {
		return fmt.Errorf("could not find Kilnfile.lock at %s: run 'kiln carvel upload' first to create the BOSH release and lockfile", lockfilePath)
	}

	c.Options.Kilnfile = kilnfilePath
	kilnfile, kilnfileLock, err := c.Options.LoadKilnfiles(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to load Kilnfiles: %w", err)
	}

	if len(kilnfileLock.Releases) == 0 {
		return fmt.Errorf("no releases found in Kilnfile.lock: run 'kiln carvel upload' first")
	}
	releaseLock := kilnfileLock.Releases[0]

	tmpDir, err := os.MkdirTemp("", "carvel-publish-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	c.outLogger.Printf("Publishing Carvel tile from %s", sourcePath)
	localTarball, err := downloadCarvelRelease(c.outLogger, kilnfile, kilnfileLock, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to download release from Artifactory: %w", err)
	}

	b := carvel.NewBaker()
	if c.Options.Verbose {
		b.SetWriter(os.Stdout)
	}

	err = b.BakeFromLockfile(sourcePath, releaseLock, localTarball)
	if err != nil {
		return fmt.Errorf("failed to prepare Carvel tile from lockfile: %w", err)
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
		Description:      "Downloads the cached BOSH release from Artifactory (using credentials from the Kilnfile) and bakes a Carvel/Kubernetes tile as a .pivotal file. Run 'kiln carvel upload' first to build and cache the release. When --final is specified, creates a bake record that can be used with 'kiln carvel re-bake' for reproducible builds.",
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
