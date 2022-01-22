package component

import (
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type BoshReleaseRepositoryRecord struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

//go:generate go run github.com/pivotal-cf/kiln/internal/component/update-boshio-index bosh_io_index.yml
//go:embed bosh_io_index.yml
var boshIOIndex string

type BoshReleaseRepositoryIndex []BoshReleaseRepositoryRecord

func GetBoshReleaseRepositoryIndex(ctx context.Context) (BoshReleaseRepositoryIndex, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://raw.githubusercontent.com/bosh-io/releases/HEAD/index.yml", nil)
	if err != nil {
		return nil, fmt.Errorf("create request for BOSH.io index failed: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET BOSH.io index  failed: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, fmt.Errorf("GET BOSH.io index failed: expected status OK (200) got %s (%d)", http.StatusText(res.StatusCode), res.StatusCode)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	var index BoshReleaseRepositoryIndex
	err = yaml.NewDecoder(res.Body).Decode(index)
	if err != nil {
		return nil, fmt.Errorf("failed to parse BOSH.io index response: %w", err)
	}
	return index, nil
}

func (index BoshReleaseRepositoryIndex) FindReleaseRepos(name string) []BoshReleaseRepositoryRecord {
	var matches []BoshReleaseRepositoryRecord
	for _, r := range index {
		if r.Name == name {
			matches = append(matches, r)
		}
	}
	return matches
}

type BOSHIOReleaseSource struct {
	cargo.ReleaseSourceConfig
	Index     BoshReleaseRepositoryIndex
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
	var index BoshReleaseRepositoryIndex
	err := yaml.Unmarshal([]byte(boshIOIndex), &index)
	if err != nil {
		panic(fmt.Errorf("failed to parse embedded bosh_io_index.yml: %w", err))
	}
	return &BOSHIOReleaseSource{
		ReleaseSourceConfig: c,
		Index:               index,
		logger:              logger,
		serverURI:           customServerURI,
	}
}

func (src BOSHIOReleaseSource) ID() string        { return src.ReleaseSourceConfig.ID }
func (src BOSHIOReleaseSource) Publishable() bool { return src.ReleaseSourceConfig.Publishable }
func (src BOSHIOReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return src.ReleaseSourceConfig
}

func (src BOSHIOReleaseSource) GetMatchedRelease(requirement Spec) (Lock, error) {
	requirement = requirement.UnsetStemcell()

	for _, rr := range src.Index.FindReleaseRepos(requirement.Name) {
		fullName := rr.URL
		fullName = strings.TrimPrefix(fullName, "https://")

		exists, err := src.releaseExistOnBoshio(fullName, requirement.Version)
		if err != nil || !exists {
			continue
		}
		builtRelease := src.createReleaseRemote(requirement, fullName)
		return builtRelease, nil
	}
	return Lock{}, ErrNotFound
}

func (src BOSHIOReleaseSource) FindReleaseVersion(spec Spec) (Lock, error) {
	spec = spec.UnsetStemcell()

	constraint, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, err
	}

	for _, rr := range src.Index.FindReleaseRepos(spec.Name) {
		fullName := rr.URL
		fullName = strings.TrimPrefix(fullName, "https://")

		releaseResponses, err := src.getReleases(fullName)
		if err != nil {
			return Lock{}, err
		}

		var validReleases []releaseResponse
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
	return Lock{}, ErrNotFound
}

func (src BOSHIOReleaseSource) DownloadRelease(releaseDir string, remoteRelease Lock) (Local, error) {
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
	defer func() { _ = out.Close() }()

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

func (src BOSHIOReleaseSource) createReleaseRemote(spec Spec, fullName string) Lock {
	downloadURL := fmt.Sprintf("%s/d/%s?v=%s", src.serverURI, fullName, spec.Version)
	releaseRemote := spec.Lock()
	releaseRemote.RemoteSource = src.ID()
	releaseRemote.RemotePath = downloadURL
	return releaseRemote
}

func (src BOSHIOReleaseSource) getReleases(name string) ([]releaseResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/releases/%s", src.serverURI, name))
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
