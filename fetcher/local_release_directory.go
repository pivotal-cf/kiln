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

func (l LocalReleaseDirectory) DeleteExtraReleases(releasesDir string, extraReleases map[cargo.CompiledRelease]string) error {
	for release, path := range extraReleases {
		err := os.Remove(path)

		if err != nil {
			fmt.Printf("error removing extra release %s: %v\n", release.Name, err)
			return fmt.Errorf("failed to delete extra release %s", release.Name)
		}
	}
	return nil
}
