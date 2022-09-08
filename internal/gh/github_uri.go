package gh

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// OwnerAndRepoFromURI is from the github-release-source branch
// once that one is merged we should that one instead of this one
func OwnerAndRepoFromURI(urlStr string) (owner, repo string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to parse owner and repo name from URI %q: %w", urlStr, err)
		}
	}()
	u, err := url.Parse(urlStr)
	if err != nil {
		if !strings.HasPrefix(urlStr, "git@github.com:") {
			return owner, repo, err
		}
		u, err = url.Parse("/" + strings.TrimPrefix(urlStr, "git@github.com:"))
		if err != nil {
			return owner, repo, err
		}
		u.Host = "github.com"
	}
	if u.Host != "github.com" {
		return owner, repo, errors.New("host must be github.com")
	}
	if filepath.Ext(u.Path) == ".git" {
		u.Path = strings.TrimSuffix(u.Path, ".git")
	}
	u.Path, repo = path.Split(u.Path)
	_, owner = path.Split(strings.TrimSuffix(u.Path, "/"))
	if owner == "" || repo == "" {
		return owner, repo, errors.New("path missing expected parts")
	}
	return owner, repo, nil
}
