package history

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func unmarshalFile(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, data interface{}, filePath string) error {
	buf, err := fileAtCommit(storage, commitHash, filePath)
	if err != nil {
		return err
	}
	return decodeFile(buf, filePath, data)
}

func decodeFile(buf []byte, fileName string, data interface{}) error {
	if filepath.Base(fileName) == cargo.KilnfileFileName {
		return yaml.Unmarshal(buf, data)
	}
	switch filepath.Ext(fileName) {
	case ".yaml", ".yml", ".lock":
		return yaml.Unmarshal(buf, data)
	case ".json":
		return json.Unmarshal(buf, data)
	}
	return nil
}

func fileAtCommit(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, filePath string) ([]byte, error) {
	commit, err := object.GetCommit(storage, commitHash)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	return readBytesFromTree(storage, tree, filePath)
}

func readBytesFromTree(storage storer.EncodedObjectStorer, tree *object.Tree, filePath string) ([]byte, error) {
	entree, err := tree.FindEntry(filePath)
	if err == object.ErrEntryNotFound {
		return nil, object.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	blob, err := storage.EncodedObject(plumbing.BlobObject, entree.Hash)
	if err != nil {
		if err == object.ErrEntryNotFound {
			return nil, object.ErrFileNotFound
		}
		return nil, err
	}
	f, err := blob.Reader()
	if err != nil {
		return nil, err
	}
	defer closeAndIgnoreError(f)
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
