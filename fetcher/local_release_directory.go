package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"
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

func (l LocalReleaseDirectory) GetLocalReleases(releasesDir string) (LocalReleaseSet, error) {
	outputReleases := LocalReleaseSet{}

	rawReleases, err := l.releasesService.FromDirectories([]string{releasesDir})
	if err != nil {
		return nil, err
	}

	for _, release := range rawReleases {
		releaseManifest := release.(builder.ReleaseManifest)
		id := ReleaseID{Name: releaseManifest.Name, Version: releaseManifest.Version}

		var rel LocalRelease
		// see implementation of ReleaseManifestReader.Read for why we can assume that
		// stemcell metadata are empty strings
		if releaseManifest.StemcellOS != "" && releaseManifest.StemcellVersion != "" {
			rel = CompiledRelease{
				ID:              id,
				StemcellOS:      releaseManifest.StemcellOS,
				StemcellVersion: releaseManifest.StemcellVersion,
				Path:            filepath.Join(releasesDir, releaseManifest.File),
			}
		} else {
			rel = NewBuiltRelease(id, filepath.Join(releasesDir, releaseManifest.File), "")
		}
		outputReleases[id] = rel
	}
	return outputReleases, nil
}

func (l LocalReleaseDirectory) DeleteExtraReleases(extraReleaseSet LocalReleaseSet, noConfirm bool) error {
	var doDeletion byte

	if len(extraReleaseSet) == 0 {
		return nil
	}

	if noConfirm {
		doDeletion = 'y'
	} else {
		l.logger.Println("Warning: kiln will delete the following files:")

		for _, release := range extraReleaseSet {
			l.logger.Printf("- %s\n", release.LocalPath())
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

func (l LocalReleaseDirectory) deleteReleases(releasesToDelete LocalReleaseSet) error {
	for releaseID, release := range releasesToDelete {
		err := os.Remove(release.LocalPath())

		if err != nil {
			l.logger.Printf("error removing release %s: %v\n", releaseID.Name, err)
			return fmt.Errorf("failed to delete release %s", releaseID.Name)
		}

		l.logger.Printf("removed release %s\n", releaseID.Name)
	}

	return nil
}

func (l LocalReleaseDirectory) VerifyChecksums(downloadedReleaseSet LocalReleaseSet, kilnfileLock cargo.KilnfileLock) error {
	if len(downloadedReleaseSet) == 0 {
		return nil
	}

	l.logger.Printf("verifying checksums")

	var badReleases []string

	fs := osfs.New("")

	var errs []error
	for releaseID, release := range downloadedReleaseSet {
		expectedSum, found := findExpectedSum(releaseID, kilnfileLock.Releases)

		if !found {
			return fmt.Errorf("release %s is not in Kilnfile.lock file", releaseID.Name)
		}
		if expectedSum == "" {
			continue
		}

		sum, err := CalculateSum(release.LocalPath(), fs)
		if err != nil {
			return fmt.Errorf("error while calculating checksum: %s", err)
		}

		if expectedSum != sum {
			l.deleteReleases(LocalReleaseSet{releaseID: release})
			badReleases = append(badReleases, fmt.Sprintf("%+v", release.LocalPath()))
		}
	}

	if len(errs) > 0 {
		return multipleErrors(errs)
	}

	if len(badReleases) != 0 {
		return fmt.Errorf("These downloaded releases do not match the checksum and were removed:\n%s", strings.Join(badReleases, "\n"))
	}

	return nil
}

func findExpectedSum(release ReleaseID, desiredReleases []cargo.Release) (string, bool) {
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
