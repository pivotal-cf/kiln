package commands

import (
	"archive/zip"
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"golang.org/x/oauth2"
	"io"
	"log"

	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v50/github"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v2"
)

type OSM struct {
	outLogger *log.Logger
	component.ReleaseSource
	gc      *github.Client
	Options struct {
		flags.Standard
		NoDownload  bool   `short:"nd" long:"no-download" default:"false" description:"Do not download & zip the packages"`
		GithubToken string `short:"g" long:"github-token" description:"Auth token for fetching specified Github packages" env:"GITHUB_TOKEN"`
		Only        string `short:"o" long:"only" default:"" description:"Only download the specified package name, must be used with --url to specify package Github URL"`
		Url         string `short:"u" long:"url" default:"" description:"Github URL for package specified by --only"`
	}
}

func NewOSM(outLogger *log.Logger, rs component.ReleaseSource) *OSM {
	if rs == nil {
		rs = component.NewBOSHIOReleaseSource(cargo.ReleaseSourceConfig{}, "", outLogger)
	}
	return &OSM{
		outLogger:     outLogger,
		ReleaseSource: rs,
	}
}

func NewOSMWithGHClient(outLogger *log.Logger, rs component.ReleaseSource, githubClient *github.Client) *OSM {
	// This is called when we need to
	if rs == nil {
		rs = component.NewBOSHIOReleaseSource(cargo.ReleaseSourceConfig{}, "", outLogger)
	}
	return &OSM{
		outLogger:     outLogger,
		ReleaseSource: rs,
		gc:            githubClient,
	}
}

func getClient(token string, ctx context.Context) *github.Client {
	//go-github client needed for singlePackage() to reach out to Github
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return client
}

func (cmd *OSM) Execute(args []string) error {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return err
	}
	ctx := context.Background()

	out := map[string]osmEntry{}

	if cmd.Options.Only == "" {
		kfPath := cargo.FullKilnfilePath(cmd.Options.Kilnfile)

		kilnfile, kilnfileLock, err := cargo.GetKilnfiles(kfPath)
		if err != nil {
			return err
		}

		for _, r := range kilnfile.Releases {
			lock, err := cmd.getReleaseLockFromBOSHIO(r)
			if err != nil {
				continue
			}

			lockVersion := findVersionForRelease(r.Name, kilnfileLock.Releases)

			url := r.GitHubRepository
			if url == "" {
				url = getURLFromLock(lock)
			}

			s := fmt.Sprintf("other:%s:%s", r.Name, lockVersion)
			out[s] = osmEntry{
				Name:              r.Name,
				Version:           lockVersion,
				Repository:        "Other",
				URL:               url,
				License:           "Apache2.0",
				Interactions:      []string{"Distributed - Calling Existing Classes"},
				OtherDistribution: fmt.Sprintf("./%s-%s.zip", r.Name, lockVersion),
			}

			if cmd.Options.NoDownload {
				continue
			}

			repoPath := fmt.Sprintf("/tmp/osm/%s", r.Name)
			err = cloneGitRepo(url, repoPath, cmd.Options.GithubToken)
			if err != nil {
				return fmt.Errorf("could not clone repo %s, %s", url, err)
			}
			err = zipRepo(repoPath, fmt.Sprintf("%s-%s.zip", r.Name, lockVersion))
			if err != nil {
				return fmt.Errorf("could not zip repo at %s, %s", repoPath, err)
			}
		}
	} else {
		// assumes --only was specified
		if cmd.Options.Url == "" || !strings.Contains(cmd.Options.Url, "github.com") {
			return fmt.Errorf("--only must be specified with a Github --url, provide a --url for the Github repository of specified package")
		}
		if cmd.gc == nil {
			cmd.gc = getClient(cmd.Options.GithubToken, ctx)
		}
		entry, s, err := cmd.singlePackage(cmd.Options.Only, cmd.Options.Url, cmd.Options.NoDownload, ctx)
		out[s] = entry
		if err != nil {
			return fmt.Errorf("could not read single package for %s: %s", cmd.Options.Only, err)
		}
	}

	o, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("coudl not marshal output into yaml: %s", err)
	}

	cmd.outLogger.Printf("%s", o)

	return nil
}

func (cmd *OSM) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command reads the Kilnfile and Kilnfile.lock for a product and produces an Open Source Manager-formated manifest.",
		ShortDescription: "Print an OSM-format manifest.",
		Flags:            cmd.Options,
	}
}

func (cmd *OSM) singlePackage(name string, url string, noDownload bool, ctx context.Context) (osmEntry, string, error) {
	//setting up for API call
	splitString := strings.SplitN(url, "/", -1)
	repo := splitString[len(splitString)-1]
	owner := splitString[len(splitString)-2]

	release, _, err := cmd.gc.Repositories.GetLatestRelease(ctx, owner, repo)

	if err != nil {
		return osmEntry{}, "", fmt.Errorf("unable to find repository for: %s", release.GetName())
	}

	version := release.GetName()

	s := fmt.Sprintf("other:%s:%s", name, version)
	entry := osmEntry{
		Name:              name,
		Version:           version,
		Repository:        "Other",
		URL:               url,
		License:           "Apache2.0",
		Interactions:      []string{"Distributed - Calling Existing Classes"},
		OtherDistribution: fmt.Sprintf("./%s-%s.zip", name, version),
	}

	if !noDownload {
		repoPath := fmt.Sprintf("/tmp/osm/%s", name)
		err := cloneGitRepo(url, repoPath, cmd.Options.GithubToken)
		if err != nil {
			return osmEntry{}, "nil", fmt.Errorf("could not clone repo %s, %s", url, err)
		}
		err = zipRepo(repoPath, fmt.Sprintf("%s-%s.zip", name, release.GetName()))
		if err != nil {
			return osmEntry{}, "nil", fmt.Errorf("could not zip repo at %s, %s", repoPath, err)
		}
	}

	return entry, s, nil
}

func cloneGitRepo(url, repoPath, githubToken string) error {
	_, err := git.PlainClone(repoPath, false, &git.CloneOptions{

		Auth: &http.BasicAuth{
			Username: "kiln-generate-osm",
			Password: githubToken,
		},
		URL:   url,
		Depth: 0,
	})
	if err != nil {
		return err
	}

	return nil
}

func zipRepo(repoPath, filename string) error {
	archive, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer archive.Close()

	zw := zip.NewWriter(archive)
	defer zw.Close()

	return filepath.Walk(repoPath, zipWalker(repoPath, zw))
}

func zipWalker(repoPath string, zw *zip.Writer) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		zipPath := strings.Replace(path, repoPath, "", 1)

		f, err := zw.Create(zipPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}

		return nil
	}
}

func (cmd *OSM) getReleaseLockFromBOSHIO(cs cargo.ComponentSpec) (cargo.ComponentLock, error) {
	lock, err := cmd.FindReleaseVersion(cs, true)
	if err != nil {
		return cmd.FindReleaseVersion(specWithoutOffline(cs), true)
	}
	return lock, nil
}

func findVersionForRelease(name string, releases []cargo.ComponentLock) string {
	for _, lr := range releases {
		if lr.Name == name {
			return lr.Version
		}
	}
	return ""
}

func specWithoutOffline(cs cargo.ComponentSpec) cargo.ComponentSpec {
	name := strings.Replace(cs.Name, "offline-", "", 1)
	return cargo.ComponentSpec{
		Name:    name,
		Version: cs.Version,
	}
}

func getURLFromLock(l cargo.ComponentLock) string {
	url := strings.Replace(l.RemotePath, "https://bosh.io/d/github.com", "https://github.com", 1)
	return url[:strings.Index(url, "?")]
}

type osmEntry struct {
	Name              string   `yaml:"name"`
	Version           string   `yaml:"version"`
	Repository        string   `yaml:"repository"`
	URL               string   `yaml:"url"`
	License           string   `yaml:"license"`
	Interactions      []string `yaml:"interactions"`
	OtherDistribution string   `yaml:"other-distribution"`
}
