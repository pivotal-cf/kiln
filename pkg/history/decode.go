package history

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"gopkg.in/yaml.v2"
)

func unmarshalFile(storage storer.EncodedObjectStorer, commitHash plumbing.Hash, data any, filePath string) error {
	buf, err := fileAtCommit(storage, commitHash, filePath)
	if err != nil {
		return err
	}
	return decodeFile(buf, filePath, data)
}

//func readDataFromTree(tree *object.Tree, data any, names []string) error {
//	buf, fileName, err := readBytesFromTree(tree, names)
//	if err != nil {
//		return err
//	}
//	return decodeFile(buf, fileName, data)
//}

func decodeFile(buf []byte, fileName string, data any) error {
	if filepath.Base(fileName) == "Kilnfile" {
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

//func findTileRootsInTree(repo *git.Repository, tree *object.Tree) []string {
//	for _, sentinelFileName := range tileRootSentinelFiles {
//		_, err := tree.File(sentinelFileName)
//		if err != nil {
//			continue
//		}
//		return []string{""}
//	}
//
//	var result []string
//
//	for _, entree := range tree.Entries {
//		if strings.HasPrefix(entree.Name, ".") {
//			continue
//		}
//		if entree.Mode != filemode.Dir {
//			continue
//		}
//		child, err := repo.TreeObject(entree.Hash)
//		if err != nil {
//			continue
//		}
//		childRoots := findTileRootsInTree(repo, child)
//		for i := range childRoots {
//			childRoots[i] = filepath.Join(entree.Name, childRoots[i])
//		}
//		result = append(result, childRoots...)
//	}
//
//	return result
//}

// var releasedVersionTag = regexp.MustCompile(`^((\w+/)*)(\d+\.\d+\.\d+)$`)

//func isReleaseTag(reference *plumbing.Reference) (string, string, bool) {
//	if !reference.Name().IsTag() {
//		return "", "", false
//	}
//	isMatch := releasedVersionTag.MatchString(reference.Name().Short())
//	if !isMatch {
//		return "", "", false
//	}
//
//	matches := releasedVersionTag.FindStringSubmatch(reference.Name().Short())
//	if len(matches) > 2 {
//		return strings.TrimSuffix(matches[1], "/"), matches[len(matches)-1], true
//	}
//
//	return "", matches[len(matches)-1], true
//}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
