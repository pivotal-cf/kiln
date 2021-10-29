package commands

import (
	"fmt"
	"log"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type UpdateStemcell struct {
	Options struct {
		options.Standard
		Version     string `short:"v"  long:"version"            required:"true"    description:"desired version of stemcell"`
		ReleasesDir string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
	}
	FS                         billy.Filesystem
	MultiReleaseSourceProvider MultiReleaseSourceProvider
	Logger                     *log.Logger
}

func (s UpdateStemcell) Execute(args []string) error {
	return Kiln{
		Wrapped: s,
		KilnfileStore: KilnfileStore{
			FS: s.FS,
		},
		StatFn: s.FS.Stat,
	}.Execute(args)
}

<<<<<<< HEAD
func (update UpdateStemcell) KilnExecute(args []string, parseOps OptionsParseFunc) (cargo.KilnfileLock, error) {
	kilnfile, kilnfileLock, _, err := parseOps(args, &update.Options)
=======
func (s UpdateStemcell) KilnExecute(args []string, parseOps OptionsParseFunc) (cargo.KilnfileLock, error) {
	kilnfile, kilnfileLock, err := parseOps(args, &s.Options)
>>>>>>> c95c5849 (refactor: standardize receiver names)
	if err != nil {
		return cargo.KilnfileLock{}, err
	}

	var releaseVersionConstraint *semver.Constraints

	trimmedInputVersion := strings.TrimSpace(s.Options.Version)

	latestStemcellVersion, err := semver.NewVersion(trimmedInputVersion)
	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("Please enter a valid stemcell version to update: %w", err)
	}

	kilnStemcellVersion := kilnfile.Stemcell.Version
	releaseVersionConstraint, err = semver.NewConstraint(kilnStemcellVersion)

	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("Invalid stemcell constraint in kilnfile: %w", err)
	}

	if !releaseVersionConstraint.Check(latestStemcellVersion) {
		s.Logger.Println("Latest version does not satisfy the stemcell version constraint in kilnfile. Nothing to update.")
		return kilnfileLock, nil
	}

	currentStemcellVersion, _ := semver.NewVersion(kilnfileLock.Stemcell.Version)

	if currentStemcellVersion.Equal(latestStemcellVersion) {
		s.Logger.Println("Stemcell is up-to-date. Nothing to update for product")
		return kilnfileLock, nil
	}

	releaseSource := s.MultiReleaseSourceProvider(kilnfile, false)

	for i, rel := range kilnfileLock.Releases {
		s.Logger.Printf("Updating release %q with stemcell %s %s...", rel.Name, kilnfileLock.Stemcell.OS, trimmedInputVersion)

		remote, found, err := releaseSource.GetMatchedRelease(component.Spec{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      kilnfileLock.Stemcell.OS,
			StemcellVersion: trimmedInputVersion,
		})
		if err != nil {
			return cargo.KilnfileLock{}, fmt.Errorf("while finding release %q, encountered error: %w", rel.Name, err)
		}
		if !found {
			return cargo.KilnfileLock{}, fmt.Errorf("couldn't find release %q", rel.Name)
		}

<<<<<<< HEAD
		if remote.RemotePath == rel.RemotePath && remote.RemoteSource == rel.RemoteSource {
			update.Logger.Printf("No change for release %q\n", rel.Name)
			continue
		}

		local, err := releaseSource.DownloadRelease(update.Options.ReleasesDir, remote)
=======
		if remote.RemotePath == rel.RemotePath && remote.SourceID == rel.RemoteSource {
			s.Logger.Printf("No change for release %q\n", rel.Name)
			continue
		}

		local, err := releaseSource.DownloadRelease(s.Options.ReleasesDir, remote, fetcher.DefaultDownloadThreadCount)
>>>>>>> c95c5849 (refactor: standardize receiver names)
		if err != nil {
			return cargo.KilnfileLock{}, fmt.Errorf("while downloading release %q, encountered error: %w", rel.Name, err)
		}

		lock := &kilnfileLock.Releases[i]
		lock.SHA1 = local.SHA1
		lock.RemotePath = remote.RemotePath
		lock.RemoteSource = remote.RemoteSource
	}

	kilnfileLock.Stemcell.Version = trimmedInputVersion

	return kilnfileLock, nil
}

func (s UpdateStemcell) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Updates stemcell and release information in Kilnfile.lock",
		ShortDescription: "updates stemcell and release information in Kilnfile.lock",
		Flags:            s.Options,
	}
}
