package historic

import (
	"bytes"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var billOfMaterialFileNames = []string{"Kilnfile.lock", "assets.lock"} // tileRootSentinelFiles   = []string{"Kilnfile", "base.yml"}

func Kilnfile(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, kilnfilePath string) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	tilePath := kilnfilePath
	if filepath.Base(tilePath) == "Kilnfile" {
		tilePath = filepath.Dir(tilePath)
	}

	var lock cargo.KilnfileLock

	var err error
	for _, name := range billOfMaterialFileNames {
		err = unmarshalFile(storage, commitHash, &lock, filepath.Join(tilePath, name))
		if err == object.ErrFileNotFound {
			continue
		}
		if err != nil {
			return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
		}
		break
	}
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	var spec cargo.Kilnfile
	err = unmarshalFile(storage, commitHash, &spec, filepath.Join(tilePath, "Kilnfile"))
	if err != nil && err != object.ErrFileNotFound {
		return cargo.Kilnfile{}, lock, err
	}

	return spec, lock, nil
}

func Version(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, kilnfilePath string) (string, error) {
	tilePath := kilnfilePath
	if filepath.Base(tilePath) == "Kilnfile" {
		tilePath = filepath.Dir(tilePath)
	}
	buf, err := fileAtCommit(storage, commitHash, filepath.Join(tilePath, "version"))
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(buf)), nil
}

func Walk(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, fn func(commit *object.Commit)) error {
	c := make(map[plumbing.Hash]struct{})
	return walk(storage, commitHash, fn, c)
}

func walk(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, fn func(commit *object.Commit), c map[plumbing.Hash]struct{}) error {
	if _, visited := c[commitHash]; visited {
		return nil
	}
	c[commitHash] = struct{}{}

	commit, err := object.GetCommit(storage, commitHash)
	if err != nil {
		return err
	}
	fn(commit)
	for _, p := range commit.ParentHashes {
		if err := walk(storage, p, fn, c); err != nil {
			return err
		}
	}
	return nil
}
