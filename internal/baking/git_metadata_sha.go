package baking

import (
	"errors"
	"github.com/go-git/go-git/v5"
)

func GitMetadataSha(p string, allowDirty bool) (string, error) {
	repo, err := git.PlainOpen(p)
	if err != nil {
		return "", err
	}

	if !allowDirty {
		wt, err := repo.Worktree()
		if err != nil {
			return "", nil
		}

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
