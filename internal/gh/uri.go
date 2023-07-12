package gh

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// RepositoryOwnerAndNameFromPath is from the github-release-source branch
// once that one is merged we should that one instead of this one
func RepositoryOwnerAndNameFromPath(urlStr string) (owner, repo string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to parse owner and repo name from URI %q: %w", urlStr, err)
		}
	}()
	if strings.HasPrefix(urlStr, "git@github.com:") {
		urlStr = strings.TrimPrefix(urlStr, "git@github.com:")
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", "", err
	}
	if filepath.Ext(u.Path) == ".git" {
		u.Path = strings.TrimSuffix(u.Path, ".git")
	}
	owner, repo, found := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if !found || owner == "" || repo == "" {
		return owner, repo, errors.New("path missing expected parts")
	}
	return owner, repo, nil
}
