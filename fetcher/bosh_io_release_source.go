package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
)

var repos = []string{
	"cloudfoundry",
	"pivotal-cf",
	"cloudfoundry-incubator",
	"pivotal-cf-experimental",
	"bosh-packages",
	"cppforlife",
	"vito",
	"flavorjones",
	"xoebus",
	"dpb587",
	"jamlo",
	"concourse",
	"cf-platform-eng",
	"starkandwayne",
	"cloudfoundry-community",
	"vmware",
	"DataDog",
	"Dynatrace",
	"SAP",
	"hybris",
	"minio",
	"rakutentech",
	"frodenas",
}

var suffixes = []string{
	"-release",
	"-boshrelease",
	"-bosh-release",
	"",
}

const (
	BOSHIOReleaseSourceID = "bosh.io"
)

type BOSHIOReleaseSource struct {
	serverURI string
	logger    *log.Logger
}

func NewBOSHIOReleaseSource(logger *log.Logger, customServerURI string) *BOSHIOReleaseSource {
	if customServerURI == "" {
		customServerURI = "https://bosh.io"
	}

	return &BOSHIOReleaseSource{
		logger:    logger,
		serverURI: customServerURI,
	}
}

func (r *BOSHIOReleaseSource) Configure(kilnfile cargo.Kilnfile) {
	return
}

func (r BOSHIOReleaseSource) GetMatchedReleases(desiredReleaseSet release.ReleaseRequirementSet) ([]release.RemoteRelease, error) {
	matches := make([]release.RemoteRelease, 0)

	for rel := range desiredReleaseSet {
	found:
		for _, repo := range repos {
			for _, suf := range suffixes {
				fullName := repo + "/" + rel.Name + suf
				exists, err := r.releaseExistOnBoshio(fullName, rel.Version)
				if err != nil {
					return nil, err
				}
				if exists {
					downloadURL := fmt.Sprintf("%s/d/github.com/%s?v=%s", r.serverURI, fullName, rel.Version)
					builtRelease := release.NewBuiltRelease(
						release.ReleaseID{Name: rel.Name, Version: rel.Version},
					).WithRemote(BOSHIOReleaseSourceID, downloadURL)
					matches = append(matches, builtRelease)
					break found
				}
			}
		}
	}

	return matches, nil //no foreseen error to return to a higher level
}

func (r BOSHIOReleaseSource) DownloadReleases(releaseDir string, remoteReleases []release.RemoteRelease, downloadThreads int) (release.ReleaseWithLocationSet, error) {
	releases := make(release.ReleaseWithLocationSet)

	r.logger.Printf("downloading %d objects from bosh.io...", len(remoteReleases))

	for _, release := range remoteReleases {
		downloadURL := release.RemotePath()
		r.logger.Printf("downloading %s...\n", downloadURL)
		// Get the data
		resp, err := http.Get(downloadURL)
		if err != nil {
			return nil, err
		}

		filePath := filepath.Join(releaseDir, release.StandardizedFilename())

		out, err := os.Create(filePath)
		if err != nil {
			return nil, err
		}

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			return nil, err
		}

		releases[release.ReleaseID()] = release.WithLocalPath(filePath)
	}

	return releases, nil
}

type ResponseStatusCodeError http.Response

func (err ResponseStatusCodeError) Error() string {
	return fmt.Sprintf("response to %s %s got status %d when a success was expected", err.Request.Method, err.Request.URL, err.StatusCode)
}

func (r BOSHIOReleaseSource) releaseExistOnBoshio(name, version string) (bool, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/releases/github.com/%s", r.serverURI, name))
	if err != nil {
		return false, fmt.Errorf("bosh.io API is down with error: %w", err)
	}
	if resp.StatusCode >= 500 {
		return false, (*ResponseStatusCodeError)(resp)
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		// we don't handle redirects yet
		// also this will catch other client request errors (>= 400)
		return false, (*ResponseStatusCodeError)(resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) == "null" {
		return false, nil
	}
	var releases []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return false, err
	}
	for _, rel := range releases {
		if rel.Version == version {
			return true, nil
		}
	}
	return false, nil
}
