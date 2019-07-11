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

func (l LocalReleaseDirectory) GetLocalReleases(releasesDir string) (CompiledReleaseSet, error) {
	outputReleases := map[CompiledRelease]string{}

	rawReleases, err := l.releasesService.FromDirectories([]string{releasesDir})
	if err != nil {
		return nil, err
	}

	for _, release := range rawReleases {
		releaseManifest := release.(builder.ReleaseManifest)
		outputReleases[CompiledRelease{
			Name:            releaseManifest.Name,
			Version:         releaseManifest.Version,
			StemcellOS:      releaseManifest.StemcellOS,
			StemcellVersion: releaseManifest.StemcellVersion,
		}] = filepath.Join(releasesDir, releaseManifest.File)
	}

	return outputReleases, nil
}

func (l LocalReleaseDirectory) DeleteExtraReleases(releasesDir string, extraReleaseSet CompiledReleaseSet, noConfirm bool) error {
	var doDeletion byte

	if len(extraReleaseSet) == 0 {
		return nil
	}

	if noConfirm {
		doDeletion = 'y'
	} else {
		l.logger.Println("Warning: kiln will delete the following files:")

		for _, path := range extraReleaseSet {
			l.logger.Printf("- %s\n", path)
		}

		l.logger.Printf("Are you sure you want to delete these files? [yN]")

		fmt.Scanf("%c", &doDeletion)
	}

	if doDeletion == 'y' || doDeletion == 'Y' {
		err := l.DeleteReleases(extraReleaseSet)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l LocalReleaseDirectory) DeleteReleases(releasesToDelete CompiledReleaseSet) error {
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

func ConvertToLocalBasename(compiledRelease CompiledRelease) string {
	if compiledRelease.IsBuiltRelease() {
		return fmt.Sprintf("%s-%s.tgz", compiledRelease.Name, compiledRelease.Version)
	}
	return fmt.Sprintf("%s-%s-%s-%s.tgz", compiledRelease.Name, compiledRelease.Version, compiledRelease.StemcellOS, compiledRelease.StemcellVersion)
}

func (l LocalReleaseDirectory) VerifyChecksums(releasesDir string, downloadedReleaseSet CompiledReleaseSet, assetsLock cargo.AssetsLock) error {
	if len(downloadedReleaseSet) == 0 {
		return nil
	}

	l.logger.Printf("verifying checksums")

	var badReleases []string

	for release, _ := range downloadedReleaseSet {
		expectedSum, found := findExpectedSum(release, assetsLock.Releases)

		if !found {
			return fmt.Errorf("release %s is not in assets file", release.Name)
		}
		if expectedSum == "" {
			continue
		}

		compiledLocalBasename := ConvertToLocalBasename(release)
		completeLocalPath := filepath.Join(releasesDir, compiledLocalBasename)
		if _, err := os.Stat(completeLocalPath); os.IsNotExist(err) {
			builtLocalBasename := ConvertToLocalBasename(CompiledRelease{
				Name:    release.Name,
				Version: release.Version,
			})
			completeLocalPath = filepath.Join(releasesDir, builtLocalBasename)
		}

		sum, err := calculateSum(completeLocalPath)
		if err != nil {
			return fmt.Errorf("error while calculating checksum: %s", err)
		}

		if expectedSum != sum {
			l.DeleteReleases(CompiledReleaseSet{release: completeLocalPath})
			badReleases = append(badReleases, fmt.Sprintf("%+v", completeLocalPath))
		}
	}

	if len(badReleases) != 0 {
		return fmt.Errorf("These downloaded releases do not match the checksum and were removed:\n%s", strings.Join(badReleases, "\n"))
	}

	return nil
}

func findExpectedSum(release CompiledRelease, desiredReleases []cargo.Release) (string, bool) {
	for _, r := range desiredReleases {
		if r.Name == release.Name {
			return r.SHA1, true
		}
	}

	return "", false
}

func calculateSum(releasePath string) (string, error) {
	f, err := os.Open(releasePath)
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
