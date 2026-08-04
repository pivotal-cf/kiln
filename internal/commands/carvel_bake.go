package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type CarvelBake struct {
	outLogger *log.Logger
	errLogger *log.Logger
	Options   CarvelBakeOptions
}

type CarvelBakeOptions struct {
	flags.Standard
	SourceDirectory   string `short:"s"   long:"source-directory"   description:"path to the Carvel tile source directory (defaults to current directory)"`
	OutputFile        string `short:"o"   long:"output-file"        description:"path to where the tile will be output" required:"true"`
	Verbose           bool   `short:"v"   long:"verbose"            description:"enable verbose output"`
	ReleasesDirectory string `short:"rd"  long:"releases-directory" default:"releases" description:"path to a directory containing/receiving additional_releases tarballs"`
	SkipFetchReleases bool   `short:"sfr" long:"skip-fetch"         description:"skip fetching additional_releases; expect them already present in --releases-directory"`
	FromLockfile      bool   `long:"from-lockfile" description:"use a cached copy of the tile's OWN release from Kilnfile.lock instead of regenerating it from source (narrow, explicit opt-in — see docs)"`
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
	c.Options.Kilnfile = kilnfilePath

	var kilnfile cargo.Kilnfile
	var kilnfileLock cargo.KilnfileLock

	_, lockfileStatErr := os.Stat(c.Options.KilnfileLockPath())
	lockfilePresent := lockfileStatErr == nil

	kf, kl, loadErr := c.Options.LoadKilnfiles(nil, nil)
	if loadErr == nil {
		kilnfile = kf
		kilnfileLock = kl
	} else {
		kfOnly, kfErr := loadKilnfileOnly(c.Options.Standard)
		if kfErr != nil {
			return fmt.Errorf("failed to load Kilnfile: %w", kfErr)
		}
		kilnfile = kfOnly

		if lockfilePresent {
			return fmt.Errorf("failed to load Kilnfile.lock: %w", loadErr)
		} else if c.Options.FromLockfile {
			return fmt.Errorf("failed to load Kilnfiles (required for --from-lockfile): %w", loadErr)
		} else {
			c.outLogger.Printf("No Kilnfile.lock found — proceeding without additional_releases support")
		}
	}

	var useLockfile bool
	if c.Options.FromLockfile {
		useLockfile = true
	} else if lockfilePresent && loadErr == nil {
		if err := baker.ParseMetadata(sourcePath); err == nil {
			if _, err := kilnfileLock.FindBOSHReleaseWithName(baker.GetBoshReleaseName()); err == nil {
				useLockfile = true
			}
		}
	}

	if useLockfile {
		if len(kilnfileLock.Releases) == 0 {
			return fmt.Errorf("no releases found in Kilnfile.lock")
		}
		
		if err := baker.ParseMetadata(sourcePath); err != nil {
			return fmt.Errorf("failed to parse metadata: %w", err)
		}
		
		releaseLock, err := kilnfileLock.FindBOSHReleaseWithName(baker.GetBoshReleaseName())
		if err != nil {
			return fmt.Errorf("release %q not found in Kilnfile.lock", baker.GetBoshReleaseName())
		}

		tmpDir, tmpErr := os.MkdirTemp("", "carvel-bake-*")
		if tmpErr != nil {
			return fmt.Errorf("failed to create temp directory: %w", tmpErr)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		localTarball, dlErr := downloadCarvelRelease(c.outLogger, kilnfile, kilnfileLock, tmpDir, baker.GetBoshReleaseName())
		if dlErr != nil {
			return fmt.Errorf("failed to download release from Artifactory: %w", dlErr)
		}

		err = baker.BakeFromLockfile(sourcePath, kilnfile, kilnfileLock, releaseLock, localTarball, carvel.BakeOptions{
			SkipFetch:         c.Options.SkipFetchReleases,
			ReleasesDirectory: c.Options.ReleasesDirectory,
		})
		if err != nil {
			return fmt.Errorf("failed to prepare Carvel tile from lockfile: %w", err)
		}
	} else {
		opts := carvel.BakeOptions{
			SkipFetch:         c.Options.SkipFetchReleases,
			ReleasesDirectory: c.Options.ReleasesDirectory,
		}
		err = baker.Bake(sourcePath, kilnfile, kilnfileLock, opts)
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
