package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
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
		l.logger.Println("Warning: kiln will delete the following files:")

		for _, path := range extraReleases {
			l.logger.Printf("- %s\n", path)
		}

		l.logger.Printf("Are you sure you want to delete these files? [yN]")

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
		err := os.Remove(path)

		if err != nil {
			l.logger.Printf("error removing release %s: %v\n", release.Name, err)
			return fmt.Errorf("failed to delete release %s", release.Name)
		}

		l.logger.Printf("removed release %s\n", release.Name)
	}

	return nil
}

func ConvertToLocalBasename(compiledRelease cargo.CompiledRelease) string {
	return fmt.Sprintf("%s-%s-%s-%s.tgz", compiledRelease.Name, compiledRelease.Version, compiledRelease.StemcellOS, compiledRelease.StemcellVersion)
}

func (l LocalReleaseDirectory) VerifyChecksums(releasesDir string, downloadedReleases map[cargo.CompiledRelease]string, assetsLock cargo.AssetsLock) error {
	if len(downloadedReleases) == 0 {
		return nil
	}

	l.logger.Printf("verifying checksums")

	var badReleases []string

	for release, _ := range downloadedReleases {
		localBasename := ConvertToLocalBasename(release)
		completeLocalPath := filepath.Join(releasesDir, localBasename)

		f, err := os.Open(completeLocalPath)
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
				release: completeLocalPath,
			}

			badReleases = append(badReleases, fmt.Sprintf(
				"%+v", completeLocalPath,
			))
			l.DeleteReleases(releaseToDelete)
		}
	}

	if len(badReleases) != 0 {
		return fmt.Errorf("These downloaded releases do not match the checksum and were removed:\n%s", strings.Join(badReleases, "\n"))
	}

	return nil
}
