package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
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
		err := l.DeleteReleases(extraReleases)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l LocalReleaseDirectory) DeleteReleases(releasesToDelete map[cargo.CompiledRelease]string) error {
	for release, path := range releasesToDelete {
		fmt.Printf("going to remove release %s\n", release.Name)
		err := os.Remove(path)

		if err != nil {
			fmt.Printf("error removing release %s: %v\n", release.Name, err)
			return fmt.Errorf("failed to delete release %s", release.Name)
		}
	}

	return nil
}

func (l LocalReleaseDirectory) VerifyChecksums(downloadedReleases map[cargo.CompiledRelease]string, assetsLock cargo.AssetsLock) error {
	fmt.Printf("verifying checksums")
	for release, path := range downloadedReleases {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		h := sha1.New()
		_, err = io.Copy(h, f)
		if err != nil {
			return err
		}

		sum := hex.EncodeToString(h.Sum(nil))
		expectedSum := ""

		for _, r := range assetsLock.Releases {
			if r.Name == release.Name {
				expectedSum = r.SHA1
				break
			}
		}

		if expectedSum == "" {
			return fmt.Errorf("release %s is not in assets file", release.Name)
		}

		if expectedSum != sum {

			releaseToDelete := map[cargo.CompiledRelease]string{
				release: path,
			}

			l.DeleteReleases(releaseToDelete)
			return fmt.Errorf("download release %s does not match SHA1 %s. Got: %s. Deleting release.", release.Name, expectedSum, sum)
		}
	}
	return nil
}
