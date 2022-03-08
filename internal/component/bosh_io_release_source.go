package component

import (
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"gopkg.in/yaml.v2"

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

func parseCachedBOSHReleaseIndex() BoshReleaseRepositoryIndex {
	var index BoshReleaseRepositoryIndex
	err := yaml.Unmarshal([]byte(boshIOIndex), &index)
	if err != nil {
		panic(fmt.Errorf("failed to parse embedded bosh_io_index.yml: %w", err))
	}
	return index
}

func GetBoshReleaseRepositoryIndex(ctx context.Context) (BoshReleaseRepositoryIndex, []error) {
	var index BoshReleaseRepositoryIndex
	err := getAndParse(ctx, "https://raw.githubusercontent.com/bosh-io/releases/HEAD/index.yml", &index)
	if err != nil {
		return index, []error{err}
	}
	cached := parseCachedBOSHReleaseIndex()
	hydrateCache(cached, index)

	var errList []error
	for i, record := range index {
		if record.URL == "" || record.Name == "" {
			continue
		}
		index[i].Name, err = record.getReleaseName(ctx)
		if err != nil {
			errList = append(errList, err)
			continue
		}
	}

	return index, errList
}

func (index BoshReleaseRepositoryIndex) FindReleaseRepos(name string) BoshReleaseRepositoryIndex {
	var matches BoshReleaseRepositoryIndex
	for _, r := range index {
		if r.Name == name {
			matches = append(matches, r)
		}
	}
	return matches
}

type BOSHIOReleaseSource struct {
	cargo.ReleaseSourceConfig
	Index      BoshReleaseRepositoryIndex
	boshIOHost string
	logger     *log.Logger
}

func NewBOSHIOReleaseSource(ctx context.Context, c cargo.ReleaseSourceConfig, boshIOHost string, logger *log.Logger, refreshCache bool) *BOSHIOReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeBOSHIO {
		panic(panicMessageWrongReleaseSourceType)
	}
	if boshIOHost == "" {
		boshIOHost = "https://bosh.io"
	}
	if logger == nil {
		logger = log.New(os.Stdout, "[bosh.io release source] ", log.Default().Flags())
	}

	index := parseCachedBOSHReleaseIndex()
	if refreshCache {
		idx, errList := GetBoshReleaseRepositoryIndex(ctx)
		for _, err := range errList {
			logger.Println(err)
		}
		index = idx
	}

	return &BOSHIOReleaseSource{
		ReleaseSourceConfig: c,
		Index:               index,
		logger:              logger,
		boshIOHost:          boshIOHost,
	}
}

func (src BOSHIOReleaseSource) ID() string        { return src.ReleaseSourceConfig.ID }
func (src BOSHIOReleaseSource) Publishable() bool { return src.ReleaseSourceConfig.Publishable }
func (src BOSHIOReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return src.ReleaseSourceConfig
}

func (src BOSHIOReleaseSource) GetMatchedRelease(spec Spec) (Lock, error) {
	spec = spec.UnsetStemcell()

	for _, rr := range src.Index.FindReleaseRepos(spec.Name) {
		fullName := rr.URL
		fullName = strings.TrimPrefix(fullName, "https://")

		exists, err := src.releaseExistOnBoshIO(context.TODO(), fullName, spec.Version)
		if err != nil || !exists {
			continue
		}
		builtRelease := src.createReleaseRemote(spec, fullName)
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

		releaseResponses, err := src.getReleases(context.TODO(), fullName)
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
	downloadURL := fmt.Sprintf("%s/d/%s?v=%s", src.boshIOHost, fullName, spec.Version)
	releaseRemote := spec.Lock()
	releaseRemote.RemoteSource = src.ID()
	releaseRemote.RemotePath = downloadURL
	return releaseRemote
}

func (src BOSHIOReleaseSource) getReleases(ctx context.Context, name string) ([]releaseResponse, error) {
	var data []map[string]string
	err := getAndParse(ctx, fmt.Sprintf("%s/api/v1/releases/%s", src.boshIOHost, name), &data)

	releases := make([]releaseResponse, 0, len(data))
	for _, m := range data {
		releases = append(releases, releaseResponse{
			Version: m["version"],
			SHA:     m["sha1"],
		})
	}
	return releases, err
}

type releaseResponse struct {
	Version string `json:"version"`
	SHA     string `json:"sha1"`
}

func (src BOSHIOReleaseSource) releaseExistOnBoshIO(ctx context.Context, name, version string) (bool, error) {
	releaseResponses, err := src.getReleases(ctx, name)
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

func (record BoshReleaseRepositoryRecord) getReleaseName(ctx context.Context) (string, error) {
	u, err := url.Parse(record.URL)
	if err != nil {
		return "", fmt.Errorf("failed to parse bosh release record url: %w", err)
	}
	var configFinal struct {
		FinalName string `yaml:"final_name"`
		Name      string `yaml:"name"`
	}
	err = getAndParse(ctx, "https://"+path.Join("raw.githubusercontent.com", u.Path, "HEAD/config/final.yml"), &configFinal)
	if err != nil {
		return "", err
	}
	if configFinal.FinalName != "" {
		record.Name = configFinal.FinalName
	} else {
		record.Name = configFinal.Name
	}
	return record.Name, nil
}

func getAndParse(ctx context.Context, uri string, data interface{}) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("%s %s request creation failed: %w", request.Method, uri, err)
	}
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", request.Method, uri, err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return (*ResponseStatusCodeError)(res)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("%s %s failed to read rsponse: %w", request.Method, uri, err)
	}
	err = yaml.Unmarshal(buf, data)
	if err != nil {
		return fmt.Errorf("failed to parse %s response: %w", uri, err)
	}
	return nil
}

func hydrateCache(previous, updated BoshReleaseRepositoryIndex) {
	for i, ur := range updated {
		for _, pr := range previous {
			if ur.URL == pr.URL {
				updated[i].Name = pr.Name
				break
			}
		}
	}
}
