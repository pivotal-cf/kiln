package commands

import (
	"fmt"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/helper"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

type UpdateStemcell struct {
	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" required:"true" description:"path to Kilnfile"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
		StemcellFile   string   `short:"sf" long:"stemcell-file" description:"path to the stemcell tarball on disk"`
		ReleasesDir    string   `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
	}
	KilnfileLoader       KilnfileLoader
	ReleaseSourceFactory ReleaseSourceFactory
	Logger               *log.Logger
}

func (update UpdateStemcell) Execute(args []string) error {
	_, err := jhanda.Parse(&update.Options, args)
	if err != nil {
		return err
	}

	fmt.Println("Parsing stemcell manifest...")
	fs := helper.NewFilesystem()
	part, err := builder.NewStemcellManifestReader(fs).Read(update.Options.StemcellFile)
	if err != nil {
		return fmt.Errorf("unable to read stemcell file: %w", err) // untested
	}

	stemcellManifest := part.Metadata.(builder.StemcellManifest)
	newStemcellOS := stemcellManifest.OperatingSystem
	newStemcellVersion := stemcellManifest.Version

	kilnfile, kilnfileLock, err := update.KilnfileLoader.LoadKilnfiles(
		osfs.New(""),
		update.Options.Kilnfile,
		update.Options.VariablesFiles,
		update.Options.Variables,
	)
	if err != nil {
		return fmt.Errorf("couldn't load kilnfiles: %w", err) // untested
	}

	if kilnfileLock.Stemcell.OS == newStemcellOS &&
		kilnfileLock.Stemcell.Version == newStemcellVersion {
		update.Logger.Println("Nothing to update for product")
		return nil
	}

	releaseSource := update.ReleaseSourceFactory.ReleaseSource(kilnfile, false)

	for i, rel := range kilnfileLock.Releases {
		fmt.Printf("Updating release %q with stemcell %s %s...", rel.Name, newStemcellOS, newStemcellVersion)

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

		local, err := releaseSource.DownloadRelease(update.Options.ReleasesDir, remote, 0)
		if err != nil {
			return fmt.Errorf("while downloading release %q, encountered error: %w", rel.Name, err)
		}

		lock := &kilnfileLock.Releases[i]
		lock.SHA1 = local.SHA1
		lock.RemotePath = remote.RemotePath
		lock.RemoteSource = remote.SourceID
	}

	kilnfileLock.Stemcell.OS = newStemcellOS
	kilnfileLock.Stemcell.Version = newStemcellVersion

	kilnfileLockFile, err := os.Create(update.Options.Kilnfile + ".lock")
	if err != nil {
		return fmt.Errorf("couldn't open the Kilnfile.lock for updating: %w", err) // untested
	}
	defer kilnfileLockFile.Close()

	err = yaml.NewEncoder(kilnfileLockFile).Encode(kilnfileLock)
	if err != nil {
		return fmt.Errorf("couldn't write the updated Kilnfile.lock: %w", err) // untested
	}

	update.Logger.Println("Finished updating Kilnfile.lock")
	return nil
}

func (update UpdateStemcell) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "updates stemcell_criteria and release information in Kilnfile.lock",
		ShortDescription: "updates Kilnfile.lock with stemcell info",
		Flags:            update.Options,
	}
}
