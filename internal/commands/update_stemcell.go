package commands

import (
	"fmt"
	"log"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/internal/fetcher"
	"github.com/pivotal-cf/kiln/pkg/release"
)

type UpdateStemcell struct {
	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile"           default:"Kilnfile" description:"path to Kilnfile"`
		Version        string   `short:"v"  long:"version"            required:"true"    description:"desired version of stemcell"`
		VariablesFiles []string `short:"vf" long:"variables-file"                        description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable"                              description:"variable in key=value format"`
		ReleasesDir    string   `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
	}
	KilnfileLoader             KilnfileLoader
	MultiReleaseSourceProvider MultiReleaseSourceProvider
	Logger                     *log.Logger
}

func (update UpdateStemcell) Execute(args []string) error {
	var releaseVersionConstraint *semver.Constraints

	_, err := jhanda.Parse(&update.Options, args)
	if err != nil {
		return err
	}

	trimmedInputVersion := strings.TrimSpace(update.Options.Version)

	latestStemcellVersion, err := semver.NewVersion(trimmedInputVersion)

	if err != nil {
		return fmt.Errorf("Please enter a valid stemcell version to update: %w", err)
	}

	kilnfile, kilnfileLock, err := update.KilnfileLoader.LoadKilnfiles(
		osfs.New(""),
		update.Options.Kilnfile,
		update.Options.VariablesFiles,
		update.Options.Variables,
	)
	if err != nil {
		return fmt.Errorf("couldn't load kilnfiles: %w", err) // untested
	}

	kilnStemcellVersion := kilnfile.Stemcell.Version
	releaseVersionConstraint, err = semver.NewConstraint(kilnStemcellVersion)

	if err != nil {
		return fmt.Errorf("Invalid stemcell constraint in kilnfile: %w", err)
	}

	if !releaseVersionConstraint.Check(latestStemcellVersion) {
		update.Logger.Println("Latest version does not satisfy the stemcell version constraint in kilnfile. Nothing to update.")
		return nil
	}

	currentStemcellVersion, _ := semver.NewVersion(kilnfileLock.Stemcell.Version)

	if currentStemcellVersion.Equal(latestStemcellVersion) ||
		currentStemcellVersion.GreaterThan(latestStemcellVersion) {
		update.Logger.Println("Stemcell is up-to-date. Nothing to update for product")
		return nil
	}

	releaseSource := update.MultiReleaseSourceProvider(kilnfile, false)

	for i, rel := range kilnfileLock.Releases {
		update.Logger.Printf("Updating release %q with stemcell %s %s...", rel.Name, kilnfileLock.Stemcell.OS, trimmedInputVersion)

		remote, found, err := releaseSource.GetMatchedRelease(release.Requirement{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      kilnfileLock.Stemcell.OS,
			StemcellVersion: trimmedInputVersion,
		})
		if err != nil {
			return fmt.Errorf("while finding release %q, encountered error: %w", rel.Name, err)
		}
		if !found {
			return fmt.Errorf("couldn't find release %q", rel.Name)
		}

		if remote.RemotePath == rel.RemotePath && remote.SourceID == rel.RemoteSource {
			update.Logger.Printf("No change for release %q\n", rel.Name)
			continue
		}

		local, err := releaseSource.DownloadRelease(update.Options.ReleasesDir, remote, fetcher.DefaultDownloadThreadCount)
		if err != nil {
			return fmt.Errorf("while downloading release %q, encountered error: %w", rel.Name, err)
		}

		lock := &kilnfileLock.Releases[i]
		lock.SHA1 = local.SHA1
		lock.RemotePath = remote.RemotePath
		lock.RemoteSource = remote.SourceID
	}

	kilnfileLock.Stemcell.Version = trimmedInputVersion

	err = update.KilnfileLoader.SaveKilnfileLock(osfs.New(""), update.Options.Kilnfile, kilnfileLock)
	if err != nil {
		return err
	}

	update.Logger.Println("Finished updating Kilnfile.lock")
	return nil
}

func (update UpdateStemcell) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Updates stemcell and release information in Kilnfile.lock",
		ShortDescription: "updates stemcell and release information in Kilnfile.lock",
		Flags:            update.Options,
	}
}
