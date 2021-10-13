package baking

import "github.com/go-git/go-git/v5"

func GitMetadataSha(p string) (string, error) {
	repo, err := git.PlainOpen(p)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	return head.Hash().String(), nil
}
