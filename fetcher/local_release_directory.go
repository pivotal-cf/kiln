package fetcher

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

type LocalReleaseDirectory struct {
	releasesService baking.ReleasesService
}

func NewLocalReleaseDirectory(releasesService baking.ReleasesService) LocalReleaseDirectory {
	return LocalReleaseDirectory{
		releasesService: releasesService,
	}
}

func (l LocalReleaseDirectory) GetLocalReleases(releasesDir string) (map[cargo.CompiledRelease]string, error) {
	outputReleases := map[cargo.CompiledRelease]string{}

	rawReleases, err := l.releasesService.FromDirectories([]string{releasesDir})
	if err != nil {
		return nil, err
	}

	for _, release := range rawReleases {
		releaseManifest := release.(builder.ReleaseManifest)
		outputReleases[cargo.CompiledRelease{
			Name:            releaseManifest.Name,
			Version:         releaseManifest.Version,
			StemcellOS:      releaseManifest.StemcellOS,
			StemcellVersion: releaseManifest.StemcellVersion,
		}] = filepath.Join(releasesDir, releaseManifest.File)
	}

	return outputReleases, nil
}

func (l LocalReleaseDirectory) DeleteExtraReleases(releasesDir string, extraReleases map[cargo.CompiledRelease]string, noConfirm bool) error {
	var doDeletion byte

	if len(extraReleases) == 0 {
		return nil
	}

	if noConfirm {
		doDeletion = 'y'
	} else {
		fmt.Println("Warning: kiln will delete the following files:")

		for _, path := range extraReleases {
			fmt.Printf("- %s\n", path)
		}

		fmt.Printf("Are you sure you want to delete these files? [yN]")

		fmt.Scanf("%c", &doDeletion)
	}

	if doDeletion == 'y' || doDeletion == 'Y' {
		for release, path := range extraReleases {
			fmt.Printf("going to remove extra release %s\n", release.Name)
			err := os.Remove(path)

			if err != nil {
				fmt.Printf("error removing extra release %s: %v\n", release.Name, err)
				return fmt.Errorf("failed to delete extra release %s", release.Name)
			}
		}
	}
	return nil
}
