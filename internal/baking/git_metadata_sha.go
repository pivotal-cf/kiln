package baking

import (
	"errors"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

func GitMetadataSha(p string, allowDirty bool) (string, error) {
	repo, err := git.PlainOpenWithOptions(p, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", err
	}

	if !allowDirty {
		wt, err := repo.Worktree()
		if err != nil {
			return "", nil
		}

		excludes, err := getExcludePatterns(wt.Filesystem.Root())
		if err != nil {
			return "", err
		}
		wt.Excludes = excludes

		status, err := wt.Status()
		if err != nil {
			return "", nil
		}

		if !status.IsClean() {
			return "", errors.New("worktree is not clean")
		}
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	return head.Hash().String(), nil
}

func getExcludePatterns(p string) ([]gitignore.Pattern, error) {
	var ignored []gitignore.Pattern

	parsePatterns := func(pth string) error {
		buf, err := ioutil.ReadFile(pth)
		if err != nil {
			return err
		}
		lines := strings.Split(string(buf), "\n")
		for i := range lines {
			ignored = append(ignored, gitignore.ParsePattern(lines[i], strings.Split(pth, string(filepath.Separator))))
		}
		return nil
	}

	// TODO: read git config --global core.excludesfile

	return ignored, filepath.Walk(p, func(pth string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if info.Name() != ".gitignore" {
			return nil
		}
		return parsePatterns(pth)
	})
}
