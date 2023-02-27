package builder

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

func GitMetadataSHA(p string, isDev bool) func() (string, error) {
	var cache string
	return func() (s string, err error) {
		if cache != "" {
			return cache, nil
		}

		repo, err := git.PlainOpenWithOptions(p, &git.PlainOpenOptions{DetectDotGit: true})
		if err != nil {
			return "", err
		}

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
			if isDev {
				cache = plumbing.ZeroHash.String()
				return cache, nil
			}
			return "", errors.New("worktree is not clean")
		}

		head, err := repo.Head()
		if err != nil {
			return "", err
		}

		cache = head.Hash().String()
		return cache, nil
	}
}

func getExcludePatterns(p string) ([]gitignore.Pattern, error) {
	var ignored []gitignore.Pattern

	parsePatterns := func(pth string) error {
		buf, err := os.ReadFile(pth)
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
