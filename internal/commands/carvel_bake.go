package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

type CarvelBake struct {
	outLogger *log.Logger
	errLogger *log.Logger
	Options   CarvelBakeOptions
}

type CarvelBakeOptions struct {
	flags.Standard
	SourceDirectory string `short:"s" long:"source-directory" description:"path to the Carvel tile source directory (defaults to current directory)"`
	OutputFile      string `short:"o" long:"output-file"      description:"path to where the tile will be output" required:"true"`
	Verbose         bool   `short:"v" long:"verbose"           description:"enable verbose output"`
}

func NewCarvelBake(outLogger, errLogger *log.Logger) CarvelBake {
	return CarvelBake{
		outLogger: outLogger,
		errLogger: errLogger,
	}
}

func (c CarvelBake) Execute(args []string) error {
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

	baker := carvel.NewBaker()
	baker.SetProgressWriter(os.Stdout)
	if c.Options.Verbose {
		baker.SetWriter(os.Stdout)
	}

	kilnfilePath := resolveKilnfilePath(c.Options.Kilnfile, sourcePath)
	lockfilePath := kilnfilePath + ".lock"
	if _, statErr := os.Stat(lockfilePath); statErr == nil {
		c.Options.Kilnfile = kilnfilePath
		kilnfile, kilnfileLock, loadErr := c.Options.LoadKilnfiles(nil, nil)
		if loadErr != nil {
			return fmt.Errorf("failed to load Kilnfiles: %w", loadErr)
		}

		if len(kilnfileLock.Releases) == 0 {
			return fmt.Errorf("no releases found in Kilnfile.lock")
		}
		releaseLock := kilnfileLock.Releases[0]

		tmpDir, tmpErr := os.MkdirTemp("", "carvel-bake-*")
		if tmpErr != nil {
			return fmt.Errorf("failed to create temp directory: %w", tmpErr)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		localTarball, dlErr := downloadCarvelRelease(c.outLogger, kilnfile, kilnfileLock, tmpDir)
		if dlErr != nil {
			return fmt.Errorf("failed to download release from Artifactory: %w", dlErr)
		}

		err = baker.BakeFromLockfile(sourcePath, releaseLock, localTarball)
		if err != nil {
			return fmt.Errorf("failed to prepare Carvel tile from lockfile: %w", err)
		}
	} else {
		err = baker.Bake(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to prepare Carvel tile: %w", err)
		}
	}

	v, err := baker.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get tile version: %w", err)
	}

	err = baker.KilnBake(targetPath)
	if err != nil {
		return fmt.Errorf("failed to bake tile: %w", err)
	}

	c.outLogger.Printf("Done! Baked %s version %s to %s", baker.GetName(), v, targetPath)
	return nil
}

func (c CarvelBake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Bakes a Carvel/Kubernetes tile into a .pivotal file. This command transforms a Kubernetes tile (using imgpkg bundles and Carvel packages) into a BOSH-compatible format, then bakes it into a .pivotal file that can be consumed by Operations Manager. When a Kilnfile.lock is present, it downloads the cached BOSH release from Artifactory instead of regenerating it locally.",
		ShortDescription: "bakes a Carvel/Kubernetes tile",
		Flags:            c.Options,
	}
}
