package component

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// OwnerAndRepoFromGitHubURI is from the github-release-source branch
// once that one is merged we should that one instead of this one
func OwnerAndRepoFromGitHubURI(urlStr string) (owner, repo string) {
	u, err := url.Parse(urlStr)
	if err != nil {
		if !strings.HasPrefix(urlStr, "git@github.com:") {
			return
		}
		u, err = url.Parse("/" + strings.TrimPrefix(urlStr, "git@github.com:"))
		if err != nil {
			return
		}
		u.Host = "github.com"
	}
	if u.Host != "github.com" {
		return
	}
	if filepath.Ext(u.Path) == ".git" {
		u.Path = strings.TrimSuffix(u.Path, ".git")
	}
	u.Path, repo = path.Split(u.Path)
	_, owner = path.Split(strings.TrimSuffix(u.Path, "/"))
	return owner, repo
}
