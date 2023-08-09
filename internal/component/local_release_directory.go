package component

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type LocalReleaseDirectory struct {
	logger *log.Logger
}

func NewLocalReleaseDirectory(logger *log.Logger) LocalReleaseDirectory {
	return LocalReleaseDirectory{
		logger: logger,
	}
}

func (l LocalReleaseDirectory) GetLocalReleases(releasesDir string) ([]Local, error) {
	if _, err := os.Stat(releasesDir); err != nil {
		return nil, fmt.Errorf("could not find releases in %s: %w", releasesDir, err)
	}
	releaseFilePaths, err := filepath.Glob(filepath.Join(releasesDir, "*.tgz"))
	if err != nil {
		return nil, err
	}

	var outputReleases []Local
	for _, releaseFilepath := range releaseFilePaths {
		releaseTarball, err := cargo.ReadBOSHReleaseTarball(releaseFilepath)
		if err != nil {
			return nil, err
		}

		lock := cargo.BOSHReleaseTarballLock{
			Name:    releaseTarball.Manifest.Name,
			Version: releaseTarball.Manifest.Version,
			SHA1:    releaseTarball.SHA1,
		}

		stemcellOS, stemcellVersion, ok := releaseTarball.Manifest.Stemcell()
		if ok {
			lock.StemcellOS = stemcellOS
			lock.StemcellVersion = stemcellVersion
		}

		outputReleases = append(outputReleases, Local{Lock: lock, LocalPath: releaseFilepath})
	}
	return outputReleases, nil
}

func (l LocalReleaseDirectory) DeleteExtraReleases(extraReleaseSet []Local, noConfirm bool) error {
	var doDeletion byte

	if len(extraReleaseSet) == 0 {
		return nil
	}

	if noConfirm {
		doDeletion = 'y'
	} else {
		l.logger.Println("Warning: kiln will delete the following files:")

		sort.SliceStable(extraReleaseSet, func(i, j int) bool {
			return extraReleaseSet[i].LocalPath < extraReleaseSet[j].LocalPath
		})

		for _, release := range extraReleaseSet {
			l.logger.Printf("- %s\n", release.LocalPath)
		}

		l.logger.Printf("Are you sure you want to delete these files? [yN]")

		_, _ = fmt.Scanf("%c", &doDeletion)
	}

	if doDeletion == 'y' || doDeletion == 'Y' {
		err := l.deleteReleases(extraReleaseSet)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l LocalReleaseDirectory) deleteReleases(releasesToDelete []Local) error {
	for _, release := range releasesToDelete {
		err := os.Remove(release.LocalPath)
		if err != nil {
			l.logger.Printf("error removing release %s: %v\n", release.Lock.Name, err)
			return fmt.Errorf("failed to delete release %s", release.Lock.Name)
		}

		l.logger.Printf("removed release %s\n", release.Lock.Name)
	}

	return nil
}

func CalculateSum(releasePath string, fs billy.Filesystem) (string, error) {
	f, err := fs.Open(releasePath)
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreError(f)

	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
