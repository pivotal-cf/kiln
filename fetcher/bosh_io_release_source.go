package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	release2 "github.com/pivotal-cf/kiln/pkg/release"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver"
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
	id          string
	serverURI   string
	publishable bool
	logger      *log.Logger
}

func NewBOSHIOReleaseSource(id string, publishable bool, customServerURI string, logger *log.Logger) *BOSHIOReleaseSource {
	if customServerURI == "" {
		customServerURI = "https://bosh.io"
	}

	return &BOSHIOReleaseSource{
		logger:      logger,
		serverURI:   customServerURI,
		publishable: publishable,
		id:          id,
	}
}

func (src BOSHIOReleaseSource) ID() string {
	return src.id
}

func (src BOSHIOReleaseSource) Publishable() bool {
	return src.publishable
}

func (src *BOSHIOReleaseSource) Configure(kilnfile cargo2.Kilnfile) {
	return
}

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement release2.Requirement) (release2.Remote, bool, error) {
	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
			if err != nil {
				return release2.Remote{}, false, err
			}

			if exists {
				builtRelease := src.createReleaseRemote(requirement.Name, requirement.Version, fullName)
				return builtRelease, true, nil
			}
		}
	}
	return release2.Remote{}, false, nil
}

func (src BOSHIOReleaseSource) FindReleaseVersion(requirement release2.Requirement) (release2.Remote, bool, error) {
	var constraint *semver.Constraints
	if requirement.VersionConstraint != "" {
		constraint, _ = semver.NewConstraint(requirement.VersionConstraint)
	} else {
		constraint, _ = semver.NewConstraint(">0")
	}
	var validReleases []releaseResponse

	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			releaseResponses, err := src.getReleases(fullName)
			if err != nil {
				return release2.Remote{}, false, err
			}

			for _, release := range releaseResponses {
				version, _ := semver.NewVersion(release.Version)
				if constraint.Check(version) {
					validReleases = append(validReleases, release)
				}
			}
			if len(validReleases) > 0 {
				latestReleaseVersion := validReleases[0].Version
				latestSha := validReleases[0].SHA
				builtRelease := src.createReleaseRemote(requirement.Name, latestReleaseVersion, fullName)
				builtRelease.SHA = latestSha
				return builtRelease, true, nil
			}
		}
	}
	return release2.Remote{}, false, nil
}

func (src BOSHIOReleaseSource) DownloadRelease(releaseDir string, remoteRelease release2.Remote, downloadThreads int) (release2.Local, error) {
	src.logger.Printf("downloading %s %s from %s", remoteRelease.Name, remoteRelease.Version, src.ID())

	downloadURL := remoteRelease.RemotePath

	resp, err := http.Get(downloadURL)
	if err != nil {
		return release2.Local{}, err
	}

	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	out, err := os.Create(filePath)
	if err != nil {
		return release2.Local{}, err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	resp.Body.Close()
	if err != nil {
		return release2.Local{}, err
	}

	_, err = out.Seek(0, 0)
	if err != nil {
		return release2.Local{}, fmt.Errorf("error reseting file cursor: %w", err) // untested
	}

	hash := sha1.New()
	_, err = io.Copy(hash, out)
	if err != nil {
		return release2.Local{}, fmt.Errorf("error hashing file contents: %w", err) // untested
	}

	sha1 := hex.EncodeToString(hash.Sum(nil))

	return release2.Local{ID: remoteRelease.ID, LocalPath: filePath, SHA1: sha1}, nil
}

type ResponseStatusCodeError http.Response

func (err ResponseStatusCodeError) Error() string {
	return fmt.Sprintf("response to %s %s got status %d when a success was expected", err.Request.Method, err.Request.URL, err.StatusCode)
}

func (src BOSHIOReleaseSource) createReleaseRemote(name string, version string, fullName string) release2.Remote {
	downloadURL := fmt.Sprintf("%s/d/github.com/%s?v=%s", src.serverURI, fullName, version)
	releaseRemote := release2.Remote{
		ID:         release2.ID{Name: name, Version: version},
		RemotePath: downloadURL,
		SourceID:   src.ID(),
	}
	return releaseRemote
}

func (src BOSHIOReleaseSource) getReleases(name string) ([]releaseResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/releases/github.com/%s", src.serverURI, name))
	if err != nil {
		return nil, fmt.Errorf("bosh.io API is down with error: %w", err)
	}
	if resp.StatusCode >= 500 {
		return nil, (*ResponseStatusCodeError)(resp)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		// we don't handle redirects yet
		// also this will catch other client request errors (>= 400)
		return nil, (*ResponseStatusCodeError)(resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) == "null" {
		return nil, nil
	}
	var releases []releaseResponse
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	return releases, nil
}

type releaseResponse struct {
	Version string `json:"version"`
	SHA     string `json:"sha1"`
}

func (src BOSHIOReleaseSource) releaseExistOnBoshio(name, version string) (bool, error) {

	releaseResponses, err := src.getReleases(name)
	if err != nil {
		return false, err
	}
	for _, rel := range releaseResponses {
		if rel.Version == version {
			return true, nil
		}
	}
	return false, nil
}
