package history

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"path/filepath"
)

func KilnfileLock(repo *git.Repository, commitHash plumbing.Hash, kilnfilePath string) (cargo.KilnfileLock, error) {
	tilePath := kilnfilePath
	if filepath.Base(tilePath) == "Kilnfile" {
		tilePath = filepath.Dir(tilePath)
	}

	var data cargo.KilnfileLock

	err := decodeHistoricFile(repo, commitHash, &data, prefixEach(tilePath, billOfMaterialFileNames)...)
	if err != nil {
		return cargo.KilnfileLock{}, err
	}

	return data, nil
}
