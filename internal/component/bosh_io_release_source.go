package component

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

	"github.com/Masterminds/semver"

	"github.com/pivotal-cf/kiln/pkg/cargo"
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

func (src *BOSHIOReleaseSource) Configure(kilnfile cargo.Kilnfile) {}

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement Requirement) (Lock, bool, error) {
	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
			if err != nil {
				return Lock{}, false, err
			}

			if exists {
				builtRelease := src.createReleaseRemote(requirement.Name, requirement.Version, fullName)
				return builtRelease, true, nil
			}
		}
	}
	return Lock{}, false, nil
}

func (src BOSHIOReleaseSource) FindReleaseVersion(requirement Requirement) (Lock, bool, error) {
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
				return Lock{}, false, err
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
				builtRelease.SHA1 = latestSha
				return builtRelease, true, nil
			}
		}
	}
	return Lock{}, false, nil
}

func (src BOSHIOReleaseSource) DownloadRelease(releaseDir string, remoteRelease Lock, downloadThreads int) (Local, error) {
	src.logger.Printf("downloading %s %s from %s", remoteRelease.Name, remoteRelease.Version, src.ID())

	downloadURL := remoteRelease.RemotePath

	resp, err := http.Get(downloadURL)
	if err != nil {
		return Local{}, err
	}

	filePath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remoteRelease.Name, remoteRelease.Version))

	out, err := os.Create(filePath)
	if err != nil {
		return Local{}, err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	resp.Body.Close()
	if err != nil {
		return Local{}, err
	}

	_, err = out.Seek(0, 0)
	if err != nil {
		return Local{}, fmt.Errorf("error reseting file cursor: %w", err) // untested
	}

	hash := sha1.New()
	_, err = io.Copy(hash, out)
	if err != nil {
		return Local{}, fmt.Errorf("error hashing file contents: %w", err) // untested
	}

	sha1 := hex.EncodeToString(hash.Sum(nil))

	return Local{Spec: remoteRelease.ComponentSpec, LocalPath: filePath, SHA1: sha1}, nil
}

type ResponseStatusCodeError http.Response

func (err ResponseStatusCodeError) Error() string {
	return fmt.Sprintf("response to %s %s got status %d when a success was expected", err.Request.Method, err.Request.URL, err.StatusCode)
}

func (src BOSHIOReleaseSource) createReleaseRemote(name string, version string, fullName string) Lock {
	downloadURL := fmt.Sprintf("%s/d/github.com/%s?v=%s", src.serverURI, fullName, version)
	releaseRemote := Lock{
		ComponentSpec: Spec{Name: name, Version: version},
		RemotePath:    downloadURL,
		RemoteSource:  src.ID(),
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
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
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
