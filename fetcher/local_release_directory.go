package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	release "github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"io"
	"log"
	"os"
)

type LocalReleaseDirectory struct {
	logger          *log.Logger
	releasesService baking.ReleasesService
}

func NewLocalReleaseDirectory(logger *log.Logger, releasesService baking.ReleasesService) LocalReleaseDirectory {
	return LocalReleaseDirectory{
		logger:          logger,
		releasesService: releasesService,
	}
}

func (l LocalReleaseDirectory) GetLocalReleases(releasesDir string) ([]release.Local, error) {
	var outputReleases []release.Local

	rawReleases, err := l.releasesService.ReleasesInDirectory(releasesDir)
	if err != nil {
		return nil, err
	}

	for _, rel := range rawReleases {
		releaseManifest := rel.Metadata.(builder.ReleaseManifest)
		id := release.ID{Name: releaseManifest.Name, Version: releaseManifest.Version}
		localPath := rel.File
		sha1, err := CalculateSum(localPath, osfs.New(""))

		if err != nil {
			return nil, fmt.Errorf("couldn't calculate SHA1 sum of %q: %w", localPath, err) // untested
		}

		outputReleases = append(outputReleases, release.Local{ID: id, LocalPath: localPath, SHA1: sha1})
	}
	return outputReleases, nil
}

func (l LocalReleaseDirectory) DeleteExtraReleases(extraReleaseSet []release.Local, noConfirm bool) error {
	var doDeletion byte

	if len(extraReleaseSet) == 0 {
		return nil
	}

	if noConfirm {
		doDeletion = 'y'
	} else {
		l.logger.Println("Warning: kiln will delete the following files:")

		for _, release := range extraReleaseSet {
			l.logger.Printf("- %s\n", release.LocalPath)
		}

		l.logger.Printf("Are you sure you want to delete these files? [yN]")

		fmt.Scanf("%c", &doDeletion)
	}

	if doDeletion == 'y' || doDeletion == 'Y' {
		err := l.deleteReleases(extraReleaseSet)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l LocalReleaseDirectory) deleteReleases(releasesToDelete []release.Local) error {
	for _, release := range releasesToDelete {
		err := os.Remove(release.LocalPath)

		if err != nil {
			l.logger.Printf("error removing release %s: %v\n", release.Name, err)
			return fmt.Errorf("failed to delete release %s", release.Name)
		}

		l.logger.Printf("removed release %s\n", release.Name)
	}

	return nil
}

func CalculateSum(releasePath string, fs billy.Filesystem) (string, error) {
	f, err := fs.Open(releasePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
