package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
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

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement release.Requirement) (release.Remote, bool, error) {
	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
			if err != nil {
				return release.Remote{}, false, err
			}

			if exists {
				downloadURL := fmt.Sprintf("%s/d/github.com/%s?v=%s", src.serverURI, fullName, requirement.Version)
				builtRelease := release.Remote{
					ID:         release.ID{Name: requirement.Name, Version: requirement.Version},
					RemotePath: downloadURL,
					SourceID:   src.ID(),
				}
				return builtRelease, true, nil
			}
		}
	}
	return release.Remote{}, false, nil
}

func (src BOSHIOReleaseSource) DownloadRelease(releaseDir string, remoteRelease release.Remote, downloadThreads int) (release.Local, error) {
	src.logger.Printf("downloading %s %s from %s", remoteRelease.Name, remoteRelease.Version, src.ID())

	downloadURL := remoteRelease.RemotePath

	resp, err := http.Get(downloadURL)
	if err != nil {
		return release.Local{}, err
	}

	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	out, err := os.Create(filePath)
	if err != nil {
		return release.Local{}, err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	resp.Body.Close()
	if err != nil {
		return release.Local{}, err
	}

	_, err = out.Seek(0, 0)
	if err != nil {
		return release.Local{}, fmt.Errorf("error reseting file cursor: %w", err) // untested
	}

	hash := sha1.New()
	_, err = io.Copy(hash, out)
	if err != nil {
		return release.Local{}, fmt.Errorf("error hashing file contents: %w", err) // untested
	}

	sha1 := hex.EncodeToString(hash.Sum(nil))

	return release.Local{ID: remoteRelease.ID, LocalPath: filePath, SHA1: sha1}, nil
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
