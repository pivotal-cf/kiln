package historic

import (
	"bytes"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var (
	billOfMaterialFileNames = []string{"Kilnfile.lock", "assets.lock"}
	// tileRootSentinelFiles   = []string{"Kilnfile", "base.yml"}
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

func Walk(repo *git.Repository, commitHash plumbing.Hash, fn func(commit *object.Commit)) error {
	c := make(map[plumbing.Hash]struct{})
	return walk(repo, commitHash, fn, c)
}

func walk(repo *git.Repository, commitHash plumbing.Hash, fn func(commit *object.Commit), c map[plumbing.Hash]struct{}) error {
	if _, visited := c[commitHash]; visited {
		return nil
	}
	c[commitHash] = struct{}{}

	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return err
	}
	fn(commit)
	for _, p := range commit.ParentHashes {
		if err := walk(repo, p, fn, c); err != nil {
			return err
		}
	}
	return nil
}
