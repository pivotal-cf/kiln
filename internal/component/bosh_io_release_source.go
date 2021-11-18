package component

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

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
		logger = log.New(os.Stdout, "[bosh.io release source] ", log.Default().Flags())
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

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement Spec) (Lock, bool, error) {
	requirement = requirement.UnsetStemcell()

	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + requirement.Name + suf
			exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
			if err != nil {
				return Lock{}, false, err
			}

			if exists {
				builtRelease := src.createReleaseRemote(requirement, fullName)
				return builtRelease, true, nil
			}
		}
	}
	return Lock{}, false, nil
}

func (src BOSHIOReleaseSource) FindReleaseVersion(spec Spec) (Lock, bool, error) {
	spec = spec.UnsetStemcell()

	constraint, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, false, err
	}

	var validReleases []releaseResponse

	for _, repo := range repos {
		for _, suf := range suffixes {
			fullName := repo + "/" + spec.Name + suf
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
			if len(validReleases) == 0 {
				continue
			}
			spec.Version = validReleases[0].Version
			lock := src.createReleaseRemote(spec, fullName)
			lock.SHA1 = validReleases[0].SHA
			return lock, true, nil
		}
	}
	return Lock{}, false, nil
}

func (src BOSHIOReleaseSource) DownloadComponent(ctx context.Context, w io.Writer, remoteRelease Lock) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteRelease.RemotePath, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	err = checkStatus(http.StatusOK, res.StatusCode)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, res.Body)
	if err != nil {
		return err
	}
	return nil
}

type ResponseStatusCodeError http.Response

func (err ResponseStatusCodeError) Error() string {
	return fmt.Sprintf("response to %s %s got status %d when a success was expected", err.Request.Method, err.Request.URL, err.StatusCode)
}

func (src BOSHIOReleaseSource) createReleaseRemote(spec Spec, fullName string) Lock {
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
