package commands

import (
	"fmt"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/helper"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

type UpdateStemcellCmd struct {
		StemcellFile   string   `short:"s" long:"stemcell-file" description:"path to the stemcell tarball on disk"`
		ReleasesDir    string   `short:"r" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
		panicCommand
}

func (u UpdateStemcellCmd) Runner(deps Dependencies) (CommandRunner, error) {
	return UpdateStemcell{
		StemcellFile:                  u.StemcellFile,
		ReleasesDir: u.ReleasesDir,

		MultiReleaseSourceProvider: deps.ReleaseSourceRepo.MultiReleaseSource,
		Logger:                     deps.OutLogger,
		KilnfileLock:               deps.KilnfileLock,
		KilnfileLockPath:           deps.KilnfileLockPath,
	}, nil
}

type UpdateStemcell struct {
	StemcellFile   string   `short:"s" long:"stemcell-file" description:"path to the stemcell tarball on disk"`
	ReleasesDir    string   `short:"r" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`

	KilnfileLock               cargo.KilnfileLock
	KilnfileLockPath           string
	MultiReleaseSourceProvider MultiReleaseSourceProvider
	Logger                     *log.Logger
}

func (update UpdateStemcell) Run(_ []string) error {
	update.Logger.Println("Parsing stemcell manifest...")
	fs := helper.NewFilesystem()
	part, err := builder.NewStemcellManifestReader(fs).Read(update.StemcellFile)
	if err != nil {
		return fmt.Errorf("unable to read stemcell file: %w", err) // untested
	}

	stemcellManifest := part.Metadata.(builder.StemcellManifest)
	newStemcellOS := stemcellManifest.OperatingSystem
	newStemcellVersion := stemcellManifest.Version

	if update.KilnfileLock.Stemcell.OS == newStemcellOS &&
		update.KilnfileLock.Stemcell.Version == newStemcellVersion {
		update.Logger.Println("Nothing to update for product")
		return nil
	}

	releaseSource := update.MultiReleaseSourceProvider(false)

	for i, rel := range update.KilnfileLock.Releases {
		update.Logger.Printf("Updating release %q with stemcell %s %s...", rel.Name, newStemcellOS, newStemcellVersion)

		remote, found, err := releaseSource.GetMatchedRelease(release.Requirement{
			Name:            rel.Name,
			Version:         rel.Version,
			StemcellOS:      newStemcellOS,
			StemcellVersion: newStemcellVersion,
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

		local, err := releaseSource.DownloadRelease(update.ReleasesDir, remote, 0)
		if err != nil {
			return fmt.Errorf("while downloading release %q, encountered error: %w", rel.Name, err)
		}

		lock := &update.KilnfileLock.Releases[i]
		lock.SHA1 = local.SHA1
		lock.RemotePath = remote.RemotePath
		lock.RemoteSource = remote.SourceID
	}

	update.KilnfileLock.Stemcell.OS = newStemcellOS
	update.KilnfileLock.Stemcell.Version = newStemcellVersion

	kilnfileLockFile, err := os.Create(update.KilnfileLockPath)
	if err != nil {
		return fmt.Errorf("couldn't open the Kilnfile.lock for updating: %w", err) // untested
	}
	defer kilnfileLockFile.Close()

	err = yaml.NewEncoder(kilnfileLockFile).Encode(update.KilnfileLock)
	if err != nil {
		return fmt.Errorf("couldn't write the updated Kilnfile.lock: %w", err) // untested
	}

	update.Logger.Println("Finished updating Kilnfile.lock")
	return nil
}
