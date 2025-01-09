package component

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"

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
	cargo.ReleaseSourceConfig
	serverURI string
	logger    *log.Logger
}

func NewBOSHIOReleaseSource(c cargo.ReleaseSourceConfig, customServerURI string, logger *log.Logger) *BOSHIOReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeBOSHIO {
		panic(panicMessageWrongReleaseSourceType)
	}
	if customServerURI == "" {
		customServerURI = "https://bosh.io"
	}

	if logger == nil {
		logger = log.New(os.Stderr, "[bosh.io release source] ", log.Default().Flags())
	}

	return &BOSHIOReleaseSource{
		ReleaseSourceConfig: c,
		logger:              logger,
		serverURI:           customServerURI,
	}
}

func (src BOSHIOReleaseSource) ID() string        { return src.ReleaseSourceConfig.ID }
func (src BOSHIOReleaseSource) Publishable() bool { return src.ReleaseSourceConfig.Publishable }
func (src BOSHIOReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return src.ReleaseSourceConfig
}

func unsetStemcell(spec cargo.BOSHReleaseTarballSpecification) cargo.BOSHReleaseTarballSpecification {
	spec.StemcellOS = ""
	spec.StemcellVersion = ""
	return spec
}

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	requirement = unsetStemcell(requirement)

	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
			if err != nil {
				return cargo.BOSHReleaseTarballLock{}, err
			}

			if exists {
				builtRelease := src.createReleaseRemote(requirement, fullName)
				return builtRelease, nil
			}
		}
	}
	return cargo.BOSHReleaseTarballLock{}, ErrNotFound
}

func (src BOSHIOReleaseSource) FindReleaseVersion(spec cargo.BOSHReleaseTarballSpecification, _ bool) (cargo.BOSHReleaseTarballLock, error) {
	spec = unsetStemcell(spec)

	constraint, err := spec.VersionConstraints()
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	var validReleases []releaseResponse

	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + spec.Name + suf
			releaseResponses, err := src.getReleases(fullName)
			if err != nil {
				return cargo.BOSHReleaseTarballLock{}, err
			}

			for _, release := range releaseResponses {
				version, _ := semver.NewVersion(release.Version)
				if constraint.Check(version) {
					validReleases = append(validReleases, release)
				}
			}
			if len(validReleases) == 0 {
				continue
			}
			spec.Version = validReleases[0].Version
			lock := src.createReleaseRemote(spec, fullName)
			lock.SHA1 = validReleases[0].SHA
			return lock, nil
		}
	}
	return cargo.BOSHReleaseTarballLock{}, ErrNotFound
}

func (src BOSHIOReleaseSource) DownloadRelease(releaseDir string, remoteRelease cargo.BOSHReleaseTarballLock) (Local, error) {
	src.logger.Printf(logLineDownload, remoteRelease.Name, ReleaseSourceTypeBOSHIO, src.ID())

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
	defer closeAndIgnoreError(out)

	_, err = io.Copy(out, resp.Body)
	_ = resp.Body.Close()
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

	remoteRelease.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Local{Lock: remoteRelease, LocalPath: filePath}, nil
}

type ResponseStatusCodeError http.Response

func (err ResponseStatusCodeError) Error() string {
	return fmt.Sprintf("response to %s %s got status %d when a success was expected", err.Request.Method, err.Request.URL, err.StatusCode)
}

func (src BOSHIOReleaseSource) createReleaseRemote(spec cargo.BOSHReleaseTarballSpecification, fullName string) cargo.BOSHReleaseTarballLock {
	downloadURL := fmt.Sprintf("%s/d/github.com/%s?v=%s", src.serverURI, fullName, spec.Version)
	releaseRemote := spec.Lock()
	releaseRemote.RemoteSource = src.ID()
	releaseRemote.RemotePath = downloadURL
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
	body, err := io.ReadAll(resp.Body)
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
