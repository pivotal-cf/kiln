package historic

import (
	"encoding/json"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

func decodeHistoricFile(repository *git.Repository, commitHash plumbing.Hash, data interface{}, names []string) error {
	buf, fileName, err := fileAtCommit(repository, commitHash, names)
	if err != nil {
		return err
	}
	return decodeFile(buf, fileName, data)
}

func readDataFromTree(tree *object.Tree, data interface{}, names []string) error {
	buf, fileName, err := readBytesFromTree(tree, names)
	if err != nil {
		return err
	}
	return decodeFile(buf, fileName, data)
}

func decodeFile(buf []byte, fileName string, data interface{}) error {
	switch filepath.Ext(fileName) {
	case ".yaml", ".yml", ".lock":
		return yaml.Unmarshal(buf, data)
	case ".json":
		return json.Unmarshal(buf, data)
	}
	return nil
}

func fileAtCommit(repository *git.Repository, commitHash plumbing.Hash, names []string) ([]byte, string, error) {
	obj, err := repository.Object(plumbing.CommitObject, commitHash)
	if err != nil {
		return nil, "", err
	}
	commit, ok := obj.(*object.Commit)
	if !ok {
		return nil, "", err
	}
	tree, err := commit.Tree()
	if !ok {
		return nil, "", err
	}
	return readBytesFromTree(tree, names)
}

func readBytesFromTree(tree *object.Tree, names []string) ([]byte, string, error) {
	var (
		lock     *object.File
		fileName string
		err      error
	)
	for _, name := range names {
		fileName = name
		lock, err = tree.File(name)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, "", err
	}
	lockFile, err := lock.Reader()
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = lockFile.Close()
	}()

	buf, err := ioutil.ReadAll(lockFile)
	if err != nil {
		return nil, "", err
	}
	return buf, fileName, nil
}

func findTileRootsInTree(repo *git.Repository, tree *object.Tree) []string {
	for _, sentinelFileName := range tileRootSentinelFiles {
		_, err := tree.File(sentinelFileName)
		if err != nil {
			continue
		}
		return []string{""}
	}

	var result []string

	for _, entree := range tree.Entries {
		if strings.HasPrefix(entree.Name, ".") {
			continue
		}
		if entree.Mode != filemode.Dir {
			continue
		}
		child, err := repo.TreeObject(entree.Hash)
		if err != nil {
			continue
		}
		childRoots := findTileRootsInTree(repo, child)
		for i := range childRoots {
			childRoots[i] = filepath.Join(entree.Name, childRoots[i])
		}
		result = append(result, childRoots...)
	}

	return result
}

var releasedVersionTag = regexp.MustCompile(`^((\w+/)*)(\d+\.\d+\.\d+)$`)

func isReleaseTag(reference *plumbing.Reference) (string, string, bool) {
	if !reference.Name().IsTag() {
		return "", "", false
	}
	isMatch := releasedVersionTag.MatchString(reference.Name().Short())
	if !isMatch {
		return "", "", false
	}

	matches := releasedVersionTag.FindStringSubmatch(reference.Name().Short())
	if len(matches) > 2 {
		return strings.TrimSuffix(matches[1], "/"), matches[len(matches)-1], true
	}

	return "", matches[len(matches)-1], true
}

func prefixEach(prefix string, names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, filepath.Join(prefix, name))
	}
	return result
}
