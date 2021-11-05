package historic

import (
	"bytes"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func KilnfileLock(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error) {
	tilePath := kilnfilePath
	if filepath.Base(tilePath) == "Kilnfile" {
		tilePath = filepath.Dir(tilePath)
	}

	var data cargo.KilnfileLock

	err := decodeHistoricFile(repo, commitHash, &data, prefixEach(tilePath, billOfMaterialFileNames))
	if err != nil {
		return cargo.KilnfileLock{}, err
	}

	return data, nil
}

func Version(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (string, error) {
	tilePath := kilnfilePath
	if filepath.Base(tilePath) == "Kilnfile" {
		tilePath = filepath.Dir(tilePath)
	}
	buf, _, err := fileAtCommit(repo, commitHash, []string{filepath.Join(tilePath, "version")})
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(buf)), nil
}
