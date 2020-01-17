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

func (src BOSHIOReleaseSource) ID() string {
	return ReleaseSourceTypeBOSHIO
}

func (src *BOSHIOReleaseSource) Configure(kilnfile cargo.Kilnfile) {
	return
}

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement release.ReleaseRequirement) (release.RemoteRelease, bool, error) {
	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
			if err != nil {
				return release.RemoteRelease{}, false, err
			}
			
			if exists {
				downloadURL := fmt.Sprintf("%s/d/github.com/%s?v=%s", src.serverURI, fullName, requirement.Version)
				builtRelease := release.RemoteRelease{
					ReleaseID:  release.ReleaseID{Name: requirement.Name, Version: requirement.Version},
					RemotePath: downloadURL,
				}
				return builtRelease, true, nil
			}
		}
	}
	return release.RemoteRelease{}, false, nil
}

func (src BOSHIOReleaseSource) DownloadRelease(releaseDir string, remoteRelease release.RemoteRelease, downloadThreads int) (release.LocalRelease, error) {
	src.logger.Printf("downloading %s %s from %s", remoteRelease.Name, remoteRelease.Version, src.ID())

	downloadURL := remoteRelease.RemotePath
	// Get the data
	resp, err := http.Get(downloadURL)
	if err != nil {
		return release.LocalRelease{}, err
	}

	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	out, err := os.Create(filePath)
	if err != nil {
		return release.LocalRelease{}, err
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	resp.Body.Close()
	out.Close()
	if err != nil {
		return release.LocalRelease{}, err
	}

	return release.LocalRelease{ReleaseID: remoteRelease.ReleaseID, LocalPath: filePath}, nil
}

type ResponseStatusCodeError http.Response

func (err ResponseStatusCodeError) Error() string {
	return fmt.Sprintf("response to %s %s got status %d when a success was expected", err.Request.Method, err.Request.URL, err.StatusCode)
}

func (src BOSHIOReleaseSource) releaseExistOnBoshio(name, version string) (bool, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/releases/github.com/%s", src.serverURI, name))
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
