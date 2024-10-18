package gh

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// RepositoryOwnerAndNameFromPath is from the github-release-source branch
// once that one is merged we should that one instead of this one
func RepositoryOwnerAndNameFromPath(urlStr string) (string, string, error) {
	_, owner, repo, err := RepositoryHostOwnerAndNameFromPath(urlStr)
	return owner, repo, err
}

func RepositoryHostOwnerAndNameFromPath(urlStr string) (string, string, string, error) {
	wrapError := func(urlStr string, err error) error {
		return fmt.Errorf("failed to parse owner and repo name from URI %q: %w", urlStr, err)
	}
	if strings.HasPrefix(urlStr, "git@") {
		exp := regexp.MustCompile(`git@(?P<host>.+):(?P<owner>[^/]+)/(?P<name>.+)(\.git)?`)
		m := exp.FindStringSubmatch(urlStr)
		if m == nil {
			return "", "", "", fmt.Errorf("path missing expected parts")
		}
		host := m[exp.SubexpIndex("host")]
		owner := m[exp.SubexpIndex("owner")]
		repo := strings.TrimSuffix(m[exp.SubexpIndex("name")], ".git")
		return host, owner, repo, nil
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", "", "", wrapError(urlStr, err)
	}
	if filepath.Ext(u.Path) == ".git" {
		u.Path = strings.TrimSuffix(u.Path, ".git")
	}
	owner, repo, found := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if !found || owner == "" || repo == "" {
		return "", owner, repo, wrapError(urlStr, fmt.Errorf("path missing expected parts"))
	}
	return u.Host, owner, repo, nil
}
