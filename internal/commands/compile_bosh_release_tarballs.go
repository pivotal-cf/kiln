package commands

import (
	"context"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"

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
		Kilnfile         string `long:"kilnfile"           default:"Kilnfile" description:"path to Kilnfile"`
		ReleaseDirectory string `long:"releases-directory" default:"releases" description:"path to a directory containing release tarballs"`

		// UploadTargetID   string `long:"upload-target-id"                      description:"the ID of the release source where the built release will be uploaded"`

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

	for _, compiled := range compiledTarballs {
		unCompiledIndex := slices.IndexFunc(boshReleaseTarballs, func(releaseTarball cargo.BOSHReleaseTarball) bool {
			return releaseTarball.Manifest.Name == compiled.Manifest.Name &&
				releaseTarball.Manifest.Version == compiled.Manifest.Version
		})
		if unCompiledIndex < 0 {
			continue
		}
		_ = os.Remove(boshReleaseTarballs[unCompiledIndex].FilePath)

		lockIndex := slices.IndexFunc(kilnfileLock.Releases, func(lock cargo.BOSHReleaseTarballLock) bool {
			return lock.Name == compiled.Manifest.Name &&
				lock.Version == compiled.Manifest.Version
		})
		if lockIndex >= 0 {
			kilnfileLock.Releases[lockIndex] = cargo.BOSHReleaseTarballLock{
				Name:    compiled.Manifest.Name,
				Version: compiled.Manifest.Version,
				SHA1:    compiled.SHA1,
			}
		}
	}

	out, err := yaml.Marshal(kilnfileLock)
	if err != nil {
		return err
	}
	return os.WriteFile(cmd.Options.Kilnfile+".lock", out, 0600)
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
