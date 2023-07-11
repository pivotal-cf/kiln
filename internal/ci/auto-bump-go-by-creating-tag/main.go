// Package auto-bump-go-by-creating-tag has a program that checks that the latest kiln GitHub release
// (at least) uses the version configured buy the setup-go action.
//
// When a binary was compiled against an older Go version, it creates a tag with the same sha as the previous version
// and pushes it. The release GitHub action should handle the rest.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

func main() {
	goVersionString := runtime.Version()
	ghActionGoVersion, err := semver.NewVersion(strings.TrimPrefix(goVersionString, "go"))
	if err != nil {
		log.Fatal("failed to parse go version: %w", err)
	}

	repoOwner, repoName, slashFound := strings.Cut(os.Getenv("GITHUB_REPOSITORY"), "/")
	if !slashFound {
		log.Fatal("failed to get repository owner and repository name")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	gh := github.NewClient(tc)

	latestRepoRelease, err := latestRepositoryRelease(ctx, repoOwner, repoName, gh)
	if !slashFound {
		log.Fatalf("failed to get latest repository release: %s", err)
	}
	tagVersion, err := semver.NewVersion(latestRepoRelease.GetTagName())
	if err != nil {
		log.Printf("failed to parse tag as version %q: %s", latestRepoRelease.GetTagName(), err)
		return
	}

	releaseAssetsList, _, err := gh.Repositories.ListReleaseAssets(ctx, repoOwner, repoName, latestRepoRelease.GetID(), &github.ListOptions{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		log.Fatalf("failed to get latest repository release assets: %s", err)
	}

	for _, asset := range releaseAssetsList {
		switch {
		case asset.GetName() == "checksums.txt":
			continue
		}
		goVersion, revisionSum, err := releaseAssetGoVersion(ctx, repoOwner, repoName, gh, asset)
		if err != nil {
			log.Printf("failed to read go version for %s: %s", asset.GetName(), err)
			continue
		}
		if !goVersion.LessThan(ghActionGoVersion) {
			log.Printf("skipping Go bump %s was compiled with Go version %s the action has Go %s", asset.GetName(), goVersion.String(), ghActionGoVersion.String())
			return
		}

		newVersion := tagVersion.IncPatch().String()
		if !strings.HasPrefix(newVersion, "v") {
			newVersion = "v" + newVersion
		}

		newRepositoryTag, _, err := gh.Git.CreateTag(ctx, repoOwner, repoName, &github.Tag{
			Tag: &newVersion,
			SHA: &revisionSum,
		})
		if err != nil {
			log.Fatalf("failed to create repository tag: %s", err)
		}

		log.Printf("created tag %s", newRepositoryTag.GetURL())

		break // we only need one success
	}
}

func releaseAssetGoVersion(ctx context.Context, repoOwner, repoName string, gh *github.Client, asset *github.ReleaseAsset) (*semver.Version, string, error) {
	rc, _, err := gh.Repositories.DownloadReleaseAsset(ctx, repoOwner, repoName, asset.GetID(), http.DefaultClient)
	if err != nil {
		return nil, "", err
	}
	defer closeAndIgnoreError(rc)
	tmpFile, err := writeTempFile(rc)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	cmd := exec.Command("go", "version", "-m", tmpFile)
	cmd.Stdin = rc
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, "", errors.Join(err, errors.New(errBuf.String()))
	}

	goVersionEx := regexp.MustCompile(`: go(?P<version>\d+\.\d+.*)`)
	versionMatches := goVersionEx.FindStringSubmatch(outBuf.String())
	versionMatchIndex := goVersionEx.SubexpIndex("version")
	if versionMatchIndex >= len(versionMatches) {
		return nil, "", fmt.Errorf("failed to find version match")
	}
	versionString := versionMatches[versionMatchIndex]
	goVersion, err := semver.NewVersion(versionString)
	if err != nil {
		return nil, "", err
	}

	revisionEx := regexp.MustCompile(`(?m)vcs\.revision=(?P<revision>.+)$`)
	revisionMatches := revisionEx.FindStringSubmatch(outBuf.String())
	revisionMatchIndex := revisionEx.SubexpIndex("revision")
	if revisionMatchIndex >= len(revisionMatches) {
		return nil, "", fmt.Errorf("failed to find revision match")
	}
	revisionString := revisionMatches[revisionMatchIndex]

	return goVersion, revisionString, nil
}

func writeTempFile(r io.Reader) (string, error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreError(f)
	if _, err := io.Copy(f, r); err != nil {
		return "", errors.Join(err, os.Remove(f.Name()))
	}
	return f.Name(), nil
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}

func latestRepositoryRelease(ctx context.Context, repoOwner, repoName string, gh *github.Client) (*github.RepositoryRelease, error) {
	options := &github.ListOptions{
		Page: 1,
	}
	for options.Page >= 0 {
		list, page, err := gh.Repositories.ListReleases(ctx, repoOwner, repoName, options)
		if err != nil {
			return nil, err
		}
		for _, release := range list {
			if release.GetDraft() || release.GetPrerelease() {
				continue
			}
			return release, nil
		}
		options.Page = page.NextPage
	}
	return nil, fmt.Errorf("failed to find published non-prerelease GitHub release")
}
