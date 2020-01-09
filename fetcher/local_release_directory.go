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

	release2 "github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"

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

func (l LocalReleaseDirectory) GetLocalReleases(releasesDir string) ([]release2.SatisfyingLocalRelease, error) {
	var outputReleases []release2.SatisfyingLocalRelease

	rawReleases, err := l.releasesService.FromDirectories([]string{releasesDir})
	if err != nil {
		return nil, err
	}

	for _, release := range rawReleases {
		releaseManifest := release.(builder.ReleaseManifest)
		id := release2.ReleaseID{Name: releaseManifest.Name, Version: releaseManifest.Version}

		var rel release2.SatisfyingLocalRelease
		// see implementation of ReleaseManifestReader.Read for why we can assume that
		// stemcell metadata are empty strings
		if releaseManifest.StemcellOS != "" && releaseManifest.StemcellVersion != "" {
			rel = release2.NewLocalCompiledRelease(
				id,
				releaseManifest.StemcellOS,
				releaseManifest.StemcellVersion,
				filepath.Join(releasesDir, releaseManifest.File),
			)
		} else {
			rel = release2.NewLocalBuiltRelease(id, filepath.Join(releasesDir, releaseManifest.File))
		}
		outputReleases = append(outputReleases, rel)
	}
	return outputReleases, nil
}

func (l LocalReleaseDirectory) DeleteExtraReleases(extraReleaseSet []release2.LocalRelease, noConfirm bool) error {
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

func (l LocalReleaseDirectory) deleteReleases(releasesToDelete []release2.LocalRelease) error {
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

func (l LocalReleaseDirectory) VerifyChecksums(downloadedReleaseSet []release2.LocalRelease, kilnfileLock cargo.KilnfileLock) error {
	if len(downloadedReleaseSet) == 0 {
		return nil
	}

	l.logger.Printf("verifying checksums")

	var badReleases []string

	fs := osfs.New("")

	for _, release := range downloadedReleaseSet {
		expectedSum, found := findExpectedSum(release.ReleaseID, kilnfileLock.Releases)

		if !found {
			return fmt.Errorf("release %s is not in Kilnfile.lock file", release.Name)
		}
		if expectedSum == "" {
			continue
		}

		sum, err := CalculateSum(release.LocalPath, fs)
		if err != nil {
			return fmt.Errorf("error while calculating checksum: %s", err)
		}

		if expectedSum != sum {
			l.deleteReleases([]release2.LocalRelease{release})
			badReleases = append(badReleases, fmt.Sprintf("%+v", release.LocalPath))
		}
	}

	if len(badReleases) != 0 {
		return fmt.Errorf("These downloaded releases do not match the checksum and were removed:\n%s", strings.Join(badReleases, "\n"))
	}

	return nil
}

func findExpectedSum(release release2.ReleaseID, desiredReleases []cargo.ReleaseLock) (string, bool) {
	for _, r := range desiredReleases {
		if r.Name == release.Name {
			return r.SHA1, true
		}
	}

	return "", false
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
