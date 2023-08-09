package commands

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"golang.org/x/exp/slices"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/directorclient"
)

//counterfeiter:generate -o ./fakes/bosh_director.go --fake-name BOSHDirector github.com/cloudfoundry/bosh-cli/v7/director.Director
type CompileBOSHReleaseTarballsFunc func(_ context.Context, logger *log.Logger, boshDirector director.Director, sc cargo.Stemcell, uploadTries int, tarballs ...cargo.BOSHReleaseTarball) ([]cargo.BOSHReleaseTarball, error)

//counterfeiter:generate -o ./fakes/compile_bosh_release_tarballs.go --fake-name CompileBOSHReleaseTarballsFunc . CompileBOSHReleaseTarballsFunc

type NewDirectorFunc func(configuration directorclient.Configuration) (director.Director, error)

type CompileBOSHReleaseTarballs struct {
	Options struct {
		flags.Standard

		ReleaseDirectory string `long:"releases-directory" default:"releases" description:"path to a directory containing release tarballs"`
		UploadTargetID   string `long:"upload-target-id"                      description:"the ID of the release source where the built release will be uploaded"`

		directorclient.Configuration
	}
	CompileBOSHReleaseTarballsFunc
	NewDirectorFunc
	Logger *log.Logger
}

func NewCompileBOSHReleaseTarballs() *CompileBOSHReleaseTarballs {
	return &CompileBOSHReleaseTarballs{
		CompileBOSHReleaseTarballsFunc: cargo.CompileBOSHReleaseTarballs,
		NewDirectorFunc:                directorclient.New,
		Logger:                         log.Default(),
	}
}

func (cmd *CompileBOSHReleaseTarballs) Execute(args []string) error {
	if _, err := jhanda.Parse(&cmd.Options, args); err != nil {
		return err
	}
	kilnfileLock, releaseTarballPaths, err := loadCompileBOSHReleasesParameters(cmd.Options.Kilnfile, cmd.Options.ReleaseDirectory)
	if err != nil {
		return err
	}

	if err := cmd.Options.Configuration.SetFieldsFromEnvironment(); err != nil {
		return err
	}
	boshDirector, err := cmd.NewDirectorFunc(cmd.Options.Configuration)
	if err != nil {
		return err
	}

	info, err := boshDirector.Info()
	if err != nil {
		return err
	}
	cmd.Logger.Println("BOSH Director info: ", info.Name, info.Version, info.CPI, info.UUID)

	boshReleaseTarballs := make([]cargo.BOSHReleaseTarball, 0, len(releaseTarballPaths))
	for _, p := range releaseTarballPaths {
		t, err := cargo.ReadBOSHReleaseTarball(p)
		if err != nil {
			return err
		}
		if len(t.Manifest.CompiledPackages) == 0 {
			boshReleaseTarballs = append(boshReleaseTarballs, t)
		}
	}

	compiledTarballs, err := cmd.CompileBOSHReleaseTarballsFunc(context.Background(), log.Default(), boshDirector, kilnfileLock.Stemcell, 5, boshReleaseTarballs...)
	if err != nil {
		return err
	}

	var uploader component.ReleaseUploader = noopUploader{}
	if cmd.Options.UploadTargetID != "" {
		_, err := flags.LoadFlagsWithDefaults(&cmd.Options, args, os.Stat)
		if err != nil {
			return err
		}

		kilnfile, _, err := cmd.Options.LoadKilnfiles(osfs.New("."), nil)
		if err != nil {
			return fmt.Errorf("failed to load kilnfiles: %w", err)
		}
		index := slices.IndexFunc(kilnfile.ReleaseSources, func(config cargo.ReleaseSourceConfig) bool {
			return cmd.Options.UploadTargetID == cargo.BOSHReleaseTarballSourceID(config)
		})
		if index < 0 {
			return fmt.Errorf("failed to load release source with ID %q", cmd.Options.UploadTargetID)
		}
		source := component.ReleaseSourceFactory(kilnfile.ReleaseSources[index], log.New(io.Discard, "", 0))
		var ok bool
		uploader, ok = source.(component.ReleaseUploader)
		if !ok {
			return fmt.Errorf("upload to release source type %s not implemented", source.Configuration().Type)
		}
	}

	for _, compiled := range compiledTarballs {
		unCompiledIndex := slices.IndexFunc(boshReleaseTarballs, func(releaseTarball cargo.BOSHReleaseTarball) bool {
			return releaseTarball.Manifest.Name == compiled.Manifest.Name &&
				releaseTarball.Manifest.Version == compiled.Manifest.Version
		})
		if unCompiledIndex < 0 {
			continue
		}
		err = uploadRelease(uploader, kilnfileLock, compiled)
		if err != nil {
			return fmt.Errorf("upload to release source with ID %s not possible", cmd.Options.UploadTargetID)
		}
		_ = os.Remove(boshReleaseTarballs[unCompiledIndex].FilePath)
	}
	if cmd.Options.UploadTargetID != "" {
		out, err := yaml.Marshal(kilnfileLock)
		if err != nil {
			return err
		}
		return os.WriteFile(cmd.Options.Kilnfile+".lock", out, 0o600)
	}
	return nil
}

func (cmd *CompileBOSHReleaseTarballs) Usage() jhanda.Usage {
	return jhanda.Usage{
		ShortDescription: "compile bosh releases with a bosh director",
		Description:      "upload bosh release tarballs to a bosh director, compile the packages against the stemcell from the stemcell lock, and download the compiled tarballs",
		Flags:            cmd.Options,
	}
}

func loadCompileBOSHReleasesParameters(kilnfilePath, releasesDirectory string) (kilnfileLock cargo.KilnfileLock, releaseTarballPaths []string, err error) {
	releaseTarballPaths, err = filepath.Glob(filepath.Join(releasesDirectory, "*.tgz"))
	if len(releaseTarballPaths) == 0 {
		err = errors.Join(fmt.Errorf("no BOSH release tarballs found"), err)
	}
	if err != nil {
		return
	}

	kilnfileLock, err = cargo.ReadKilnfileLock(kilnfilePath)
	return
}

func uploadRelease(uploader component.ReleaseUploader, kilnfileLock cargo.KilnfileLock, compiled cargo.BOSHReleaseTarball) error {
	lockIndex := slices.IndexFunc(kilnfileLock.Releases, func(lock cargo.BOSHReleaseTarballLock) bool {
		return lock.Name == compiled.Manifest.Name &&
			lock.Version == compiled.Manifest.Version
	})
	if lockIndex < 0 {
		return fmt.Errorf("release %s/%s not found in Kilnfile.lock", compiled.Manifest.Name, compiled.Manifest.Version)
	}
	tarballFile, err := os.Open(compiled.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open tarball: %w", err)
	}
	defer closeAndIgnoreError(tarballFile)
	releaseLock, err := uploader.UploadRelease(cargo.BOSHReleaseTarballSpecification{
		Name:            kilnfileLock.Releases[lockIndex].Name,
		Version:         kilnfileLock.Releases[lockIndex].Version,
		StemcellOS:      kilnfileLock.Releases[lockIndex].StemcellOS,
		StemcellVersion: kilnfileLock.Releases[lockIndex].StemcellVersion,
	}, tarballFile)
	if err != nil {
		return err
	}
	kilnfileLock.Releases[lockIndex] = releaseLock
	return nil
}

type noopUploader struct{}

func (n noopUploader) GetMatchedRelease(specification cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	return cargo.BOSHReleaseTarballLock{
		Name:            specification.Name,
		Version:         specification.Version,
		StemcellOS:      specification.StemcellOS,
		StemcellVersion: specification.StemcellVersion,
	}, nil
}

func (n noopUploader) UploadRelease(specification cargo.BOSHReleaseTarballSpecification, file io.Reader) (cargo.BOSHReleaseTarballLock, error) {
	return cargo.BOSHReleaseTarballLock{
		Name:            specification.Name,
		Version:         specification.Version,
		StemcellOS:      specification.StemcellOS,
		StemcellVersion: specification.StemcellVersion,
	}, nil
}
