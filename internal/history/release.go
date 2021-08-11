// Package history provides fast utility functions for
// navigating the history of of a tile repo.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/pkg/release"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var (
	billOfMaterialFileNames = []string{"Kilnfile.lock", "assets.lock"}
	tileRootSentinelFiles   = []string{"Kilnfile", "base.yml"}
)

type ReleaseMapping struct {
	Tile release.ID
	Bosh release.ID
}

type TileVersionFileBoshReleaseListStopFunc = func(i int, commit object.Commit, result []ReleaseMapping) bool

func stopAfterAnyTrueReturn(fns []TileVersionFileBoshReleaseListStopFunc) TileVersionFileBoshReleaseListStopFunc {
	return func(i int, commit object.Commit, result []ReleaseMapping) bool {
		for _, fn := range fns {
			if fn(i, commit, result) {
				return true
			}
		}
		return false
	}
}

func StopAfter(commitHistoryLen int) TileVersionFileBoshReleaseListStopFunc {
	return func(i int, _ object.Commit, _ []ReleaseMapping) bool {
		return i > commitHistoryLen
	}
}

func FindBoshRelease(release release.ID) TileVersionFileBoshReleaseListStopFunc {
	return func(_ int, _ object.Commit, list []ReleaseMapping) bool {
		for _, m := range list {
			if release == m.Bosh {
				return true
			}
		}
		return false
	}
}

func FindBoshTileRelease(release release.ID) TileVersionFileBoshReleaseListStopFunc {
	return func(_ int, _ object.Commit, list []ReleaseMapping) bool {
		for _, m := range list {
			if release == m.Tile {
				return true
			}
		}
		return false
	}
}

func TileVersionFileBoshReleaseList(repo *git.Repository, branchFilter *regexp.Regexp, releaseNames []string, stopFns ...TileVersionFileBoshReleaseListStopFunc) ([]ReleaseMapping, error) {
	branchIter, err := repo.Branches()
	if err != nil {
		return nil, err
	}

	var result []ReleaseMapping

	iterErr := branchIter.ForEach(func(reference *plumbing.Reference) error {
		if !branchFilter.MatchString(reference.Name().Short()) {
			return nil
		}

		hash := reference.Hash()

		for i := 0; ; i++ {
			obj, err := repo.Object(plumbing.CommitObject, hash)
			if err != nil {
				break
			}
			commit, ok := obj.(*object.Commit)
			if !ok {
				break
			}
			tree, err := commit.Tree()
			if err != nil {
				return err
			}

			for _, root := range findTileRootsInTree(repo, tree) {
				mappings, err := buildNumberBoshRelease(tree, strings.TrimSuffix(root, string(filepath.Separator)), releaseNames)
				if err != nil {
					return err
				}
				result = append(result, mappings...)
			}
			if stopAfterAnyTrueReturn(stopFns)(i, *commit, result) {
				break
			}

			// iter
			if commit.NumParents() == 0 {
				break
			}
			parent, err := commit.Parent(0)
			if err != nil {
				break
			}
			hash = parent.Hash
		}

		return nil
	})

	return ensureUnique(result), iterErr
}

func buildNumberBoshRelease(tree *object.Tree, root string, releaseNames []string) ([]ReleaseMapping, error) {
	vf, err := tree.File(filepath.Join(root, "version"))
	if err != nil {
		return nil, err
	}
	vBuf, err := vf.Contents()
	if err != nil {
		return nil, err
	}

	var lock cargo.KilnfileLock

	err = readDataFromTree(tree, &lock, prefixEach(root, billOfMaterialFileNames)...)
	if err != nil {
		return nil, err
	}

	var result []ReleaseMapping

	for _, rel := range lock.Releases {
		if !contains(releaseNames, rel.Name) {
			continue
		}

		result = append(result, ReleaseMapping{
			Tile: release.ID{
				Name:    strings.TrimSpace(root),
				Version: strings.TrimSpace(vBuf),
			},
			Bosh: release.ID{
				Name:    strings.TrimSpace(rel.Name),
				Version: strings.TrimSpace(rel.Version),
			},
		})
	}

	return ensureUnique(result), nil
}

// TileReleaseBoshReleaseList iterates over repo tags and maps bosh releases in a kilnfile.lock to tile releases.
// Tile releases are expected to be marked with a tag like "tile-path/1.2.3", where "tile-path" is the path to the
// directory containing the kilnfile and "1.2.3" is the tile version. The version must be semver parsable.
func TileReleaseBoshReleaseList(repo *git.Repository, releaseNames ...string) ([]ReleaseMapping, error) {
	var result []ReleaseMapping

	mapping, tilePaths, err := mapBoshReleaseToHistoricTileReleases(releaseNames, repo)
	if err != nil {
		return nil, err
	}

	for _, boshReleaseName := range releaseNames {
		boshReleaseVersions := mapping.boshReleaseVersions(boshReleaseName)

		sort.Sort(sort.Reverse(boshReleaseVersions))

		for _, boshReleaseVersion := range boshReleaseVersions {
			for _, tilePath := range tilePaths {
				tileVersions := mapping.tileReleaseVersions(tilePath, boshReleaseName, boshReleaseVersion)

				if len(tileVersions) == 0 {
					continue
				}

				sort.Sort(sort.Reverse(tileVersions))

				for _, v := range tileVersions {
					result = append(result, ReleaseMapping{
						Bosh: release.ID{Name: boshReleaseName, Version: boshReleaseVersion.String()},
						Tile: release.ID{Name: tilePath, Version: v.String()},
					})
				}
			}
		}
	}

	return ensureUnique(result), nil
}

func lockFiles(repo *git.Repository, fn func(tileDir string, ref plumbing.Reference, lock cargo.KilnfileLock)) ([]string, error) {
	tagReferences := make(map[string][]*plumbing.Reference)

	iter, err := repo.References()
	if err != nil {
		return nil, err
	}

	_ = iter.ForEach(func(reference *plumbing.Reference) error {
		prefix, _, isMatch := isReleaseTag(reference)
		if !isMatch {
			return nil
		}

		tagReferences[prefix] = append(tagReferences[prefix], reference)
		return nil
	})

	var paths []string
	for p := range tagReferences {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for path, tagRefs := range tagReferences {
		var versions []*semver.Version

		filtered := tagRefs[:0]
		for _, r := range tagRefs {
			_, versionString, _ := isReleaseTag(r)
			v, err := semver.NewVersion(versionString)
			if err != nil {
				continue
			}

			filtered = append(filtered, r)
			versions = append(versions, v)
		}
		tagRefs = filtered

		sort.Sort(sorter{
			len(tagRefs),
			func(i, j int) {
				tagRefs[i], tagRefs[j] = tagRefs[j], tagRefs[i]
			},
			func(i, j int) bool {
				return versions[i].LessThan(versions[j])
			},
		})

		for _, ref := range tagRefs {
			if ref.Type() != plumbing.HashReference {
				fmt.Println("UNEXPECTED non-hash reference", ref)
				continue
			}
			var data cargo.KilnfileLock
			err := decodeHistoricFile(repo, ref, &data, prefixEach(path, billOfMaterialFileNames)...)
			if err != nil {
				if errors.Is(err, object.ErrFileNotFound) {
					continue
				}
				return nil, err
			}

			fn(path, *ref, data)
		}
	}

	return paths, nil
}

func decodeHistoricFile(repository *git.Repository, ref *plumbing.Reference, data interface{}, names ...string) error {
	obj, err := repository.Object(plumbing.CommitObject, ref.Hash())
	if err != nil {
		return err
	}

	commit, ok := obj.(*object.Commit)
	if !ok {
		return err
	}

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	return readDataFromTree(tree, data, names...)
}

func readDataFromTree(tree *object.Tree, data interface{}, names ...string) error {
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
		return err
	}
	lockFile, err := lock.Reader()
	if err != nil {
		return err
	}
	defer func() {
		_ = lockFile.Close()
	}()

	buf, err := ioutil.ReadAll(lockFile)
	if err != nil {
		return err
	}

	switch filepath.Ext(fileName) {
	case ".yaml", ".yml", ".lock":
		err = yaml.Unmarshal(buf, data)
	case ".json":
		err = json.Unmarshal(buf, data)
	}
	return err
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

type releaseID struct {
	Name    string
	Version string
}

func (id releaseID) Semver() *semver.Version {
	v, _ := semver.NewVersion(id.Version)
	return v
}

type boshReleaseToTileRelease map[releaseID]map[releaseID]struct{}

func mapBoshReleaseToHistoricTileReleases(releaseNames []string, repo *git.Repository) (boshReleaseToTileRelease, []string, error) {
	mapping := make(boshReleaseToTileRelease)

	tilePaths, err := lockFiles(repo, func(tilePath string, ref plumbing.Reference, lock cargo.KilnfileLock) {
		_, versionString, _ := isReleaseTag(&ref)
		tileReleaseVersion, err := semver.NewVersion(versionString)
		if err != nil {
			return
		}

		for _, releaseName := range releaseNames {
			rel, err := lock.FindReleaseWithName(releaseName)
			if err != nil {
				continue
			}

			boshReleaseVersion, err := semver.NewVersion(strings.TrimPrefix(rel.Version, "v"))
			if err != nil {
				continue
			}

			relID := releaseID{Name: rel.Name, Version: boshReleaseVersion.String()}
			tileReleases, ok := mapping[relID]
			if !ok {
				tileReleases = make(map[releaseID]struct{})
			}

			tileReleases[releaseID{Name: tilePath, Version: tileReleaseVersion.String()}] = struct{}{}

			mapping[relID] = tileReleases
		}
	})

	return mapping, tilePaths, err
}

func (mapping boshReleaseToTileRelease) boshReleaseVersions(releaseName string) semver.Collection {
	var boshReleaseVersions semver.Collection

	for boshRelease := range mapping {
		if boshRelease.Name != releaseName {
			continue
		}

		v, err := semver.NewVersion(boshRelease.Version)
		if err != nil {
			fmt.Println("error", boshRelease.Version, err)
			continue
		}

		boshReleaseVersions = append(boshReleaseVersions, v)
	}

	return boshReleaseVersions
}

func (mapping boshReleaseToTileRelease) tileReleaseVersions(tileName, boshReleaseName string, boshReleaseVersion *semver.Version) semver.Collection {
	var tileVersions semver.Collection

	tileReleases := mapping[releaseID{Name: boshReleaseName, Version: boshReleaseVersion.String()}]

	for tileID := range tileReleases {
		if tileID.Name != tileName {
			continue
		}

		v, err := semver.NewVersion(tileID.Version)
		if err != nil {
			continue
		}

		tileVersions = append(tileVersions, v)
	}

	return tileVersions
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

// Sort "generic"
// https://medium.com/capital-one-tech/closures-are-the-generics-for-go-cb32021fb5b5
type sorter struct {
	len  int
	swap func(i, j int)
	less func(i, j int) bool
}

func (x sorter) Len() int           { return x.len }
func (x sorter) Swap(i, j int)      { x.swap(i, j) }
func (x sorter) Less(i, j int) bool { return x.less(i, j) }

func prefixEach(prefix string, names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, filepath.Join(prefix, name))
	}
	return result
}

func contains(values []string, has string) bool {
	for _, v := range values {
		if has == v {
			return true
		}
	}
	return false
}

func ensureUnique(list []ReleaseMapping) []ReleaseMapping {
	vs := make(map[string]*semver.Version)
	m := make(map[ReleaseMapping]struct{}, len(list))
	for _, v := range list {
		m[v] = struct{}{}

		vs[v.Tile.Version], _ = semver.NewVersion(v.Tile.Version)
	}

	filtered := list[:0]
	for k := range m {
		filtered = append(filtered, k)
	}
	list = filtered

	sort.Sort(sorter{
		len: len(list),
		swap: func(i, j int) {
			list[i], list[j] = list[j], list[i]
		},
		less: func(i, j int) bool {
			itv := vs[list[i].Tile.Version]
			jtv := vs[list[j].Tile.Version]
			// sort by tile version
			if !itv.Equal(jtv) {
				return itv.LessThan(jtv)
			}
			if c := strings.Compare(list[i].Tile.Name, list[j].Tile.Name); c != 0 {
				return c < 0
			}
			return strings.Compare(list[i].Bosh.Name, list[j].Bosh.Name) < 0
		},
	})

	return list
}
