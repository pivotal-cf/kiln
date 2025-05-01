package component

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"

	"github.com/Masterminds/semver/v3"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const (
	// https://github.com/Masterminds/semver/blob/1558ca3488226e3490894a145e831ad58a5ff958/version.go#L44
	semverRegex = `v?(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:\.(0|[1-9]\d*))?` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?`

	reReleaseVersionGroup  = "bosh_version"
	reStemcellVersionGroup = "bosh_stemcell_version"
)

type SearchResponseFile struct {
	Repo     string    `json:"repo"`
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Size     int       `json:"size"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	SHA256   string    `json:"sha256"`
	SHA1     string    `json:"actual_sha1"`
}

type ArtifactoryReleaseSource struct {
	cargo.ReleaseSourceConfig
	Client *http.Client
	logger *log.Logger
	ID     string
}

type ArtifactoryFile struct {
	URI    string `json:"uri"`
	Folder bool   `json:"folder"`
	SHA1   string
}

// NewArtifactoryReleaseSource will provision a new ArtifactoryReleaseSource Project
// from the Kilnfile (ReleaseSourceConfig). If type is incorrect it will PANIC
func NewArtifactoryReleaseSource(c cargo.ReleaseSourceConfig, logger *log.Logger) *ArtifactoryReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeArtifactory {
		panic(panicMessageWrongReleaseSourceType)
	}

	// ctx := context.TODO()

	if logger == nil {
		logger = log.New(os.Stderr, "[Artifactory release source] ", log.Default().Flags())
	}

	return &ArtifactoryReleaseSource{
		Client:              http.DefaultClient,
		ReleaseSourceConfig: c,
		ID:                  c.ID,
		logger:              logger,
	}
}

func (ars *ArtifactoryReleaseSource) DownloadRelease(releaseDir string, remoteRelease cargo.BOSHReleaseTarballLock) (Local, error) {
	u, err := url.Parse(ars.ArtifactoryHost)
	if err != nil {
		return Local{}, fmt.Errorf("error parsing artifactory host: %w", err)
	}
	downloadURL := ars.ArtifactoryHost
	if path.Base(u.Path) != "artifactory" {
		downloadURL += "/artifactory"
	}
	downloadURL += "/" + ars.Repo + "/" + remoteRelease.RemotePath

	ars.logger.Printf(logLineDownload, remoteRelease.Name, remoteRelease.Version, ReleaseSourceTypeArtifactory, ars.ID)
	resp, err := ars.getWithAuth(downloadURL)
	if err != nil {
		return Local{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Local{}, fmt.Errorf("failed to download %s release from artifactory with error code %d", remoteRelease.Name, resp.StatusCode)
	}

	filePath := filepath.Join(releaseDir, filepath.Base(remoteRelease.RemotePath))

	out, err := os.Create(filePath)
	if err != nil {
		return Local{}, err
	}
	defer closeAndIgnoreError(out)

	hash := sha1.New()

	mw := io.MultiWriter(out, hash)
	_, err = io.Copy(mw, resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return Local{}, err
	}

	remoteRelease.SHA1 = hex.EncodeToString(hash.Sum(nil))

	return Local{Lock: remoteRelease, LocalPath: filePath}, nil
}

func (ars *ArtifactoryReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return ars.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (ars *ArtifactoryReleaseSource) GetMatchedRelease(spec cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	matchedRelease, err := ars.findReleaseVersion(spec, spec)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, wrapVPNError(err)
	}

	return matchedRelease, nil
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (ars *ArtifactoryReleaseSource) FindReleaseVersion(spec cargo.BOSHReleaseTarballSpecification, _ bool) (cargo.BOSHReleaseTarballLock, error) {
	searchSpec := spec
	searchSpec.Version = "*" // we need to look at all available versions before deciding on the best match
	foundRelease, err := ars.findReleaseVersion(spec, searchSpec)
	return foundRelease, wrapVPNError(err)
}

func (ars *ArtifactoryReleaseSource) findReleaseVersion(spec, searchSpec cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	if spec.StemcellOS != "" {
		if spec.StemcellVersion == "" {
			return cargo.BOSHReleaseTarballLock{}, errors.New("stemcell version is required when stemcell os is set")
		}
	}

	re, err := ars.regexPatternFromSpec(spec)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	constraint, err := spec.VersionConstraints()
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}
	remoteSearchPath, err := ars.RemotePath(searchSpec)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}
	artifactoryFiles, err := ars.searchAql(remoteSearchPath)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	foundRelease := cargo.BOSHReleaseTarballLock{}
	for _, artifactoryFile := range artifactoryFiles {
		if artifactoryFile.Folder {
			continue
		}
		matches := re.FindStringSubmatch(artifactoryFile.URI)
		if matches == nil {
			continue
		}
		names := re.SubexpNames()
		matchedGroups := map[string]string{}
		for i, n := range names {
			if i == 0 || n == "" {
				continue
			}
			matchedGroups[n] = matches[i]
		}

		version := matchedGroups[reReleaseVersionGroup]
		stemcellVersion := matchedGroups[reStemcellVersionGroup]
		// we aren't updating stemcell version
		if stemcellVersion != spec.StemcellVersion {
			continue
		}

		if version != "" {
			newVersion, err := semver.NewVersion(version)
			if err != nil {
				continue
			}
			if !constraint.Check(newVersion) {
				continue
			}

			if (foundRelease == cargo.BOSHReleaseTarballLock{}) {
				foundRelease = cargo.BOSHReleaseTarballLock{
					Name:         spec.Name,
					Version:      version,
					RemotePath:   artifactoryFile.URI,
					RemoteSource: ars.ReleaseSourceConfig.ID,
					SHA1:         artifactoryFile.SHA1,
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version) // foundRelease.Version was already validated
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = cargo.BOSHReleaseTarballLock{
						Name:         spec.Name,
						Version:      version,
						RemotePath:   artifactoryFile.URI,
						RemoteSource: ars.ReleaseSourceConfig.ID,
						SHA1:         artifactoryFile.SHA1,
					}
				}
			}
		}
	}

	if (foundRelease == cargo.BOSHReleaseTarballLock{}) {
		return cargo.BOSHReleaseTarballLock{}, ErrNotFound
	}

	return foundRelease, nil
}

func (ars *ArtifactoryReleaseSource) regexPatternFromSpec(spec cargo.BOSHReleaseTarballSpecification) (*regexp.Regexp, error) {
	regexSpec := spec
	regexSpec.Version = fmt.Sprintf(`(?P<%s>(%s))`, reReleaseVersionGroup, semverRegex)
	regexSpec.StemcellVersion = fmt.Sprintf(`(?P<%s>(%s))`, reStemcellVersionGroup, semverRegex)

	semverFilepathRegex, err := ars.RemotePath(regexSpec)
	if err != nil {
		return nil, err
	}

	re, err := regexp.Compile(semverFilepathRegex)
	return re, err
}

func (ars *ArtifactoryReleaseSource) RemotePath(spec cargo.BOSHReleaseTarballSpecification) (string, error) {
	pathBuf := new(bytes.Buffer)

	err := ars.pathTemplate().Execute(pathBuf, spec)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate path_template: %w", err)
	}

	return pathBuf.String(), nil
}

func (ars *ArtifactoryReleaseSource) pathTemplate() *template.Template {
	return template.Must(
		template.New("remote-path").
			Funcs(template.FuncMap{"trimSuffix": strings.TrimSuffix}).
			Parse(ars.PathTemplate))
}

func (ars *ArtifactoryReleaseSource) getWithAuth(url string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if ars.Username != "" {
		request.SetBasicAuth(ars.Username, ars.Password)
	}
	response, err := ars.Client.Do(request)
	return response, wrapVPNError(err)
}

func aqlQuery(repo, pathPattern string) string {
	pathMatcher := path.Dir(pathPattern)
	fileMatcher := path.Base(pathPattern)
	return fmt.Sprintf(`{"repo": %[1]q, "$and": [
            { "path": { "$match": %[2]q } },
            { "name": { "$match": %[3]q } }
          ]}`, repo,
		pathMatcher,
		fileMatcher)
}

func (ars *ArtifactoryReleaseSource) searchAql(pathPattern string) ([]ArtifactoryFile, error) {
	am, err := ars.buildArtifactoryServiceManager()
	if err != nil {
		return nil, err
	}

	aql := utils.Aql{ItemsFind: aqlQuery(ars.Repo, pathPattern)}
	cr, err := am.SearchFiles(services.SearchParams{
		CommonParams: &utils.CommonParams{
			Aql: aql,
		},
	})
	if err != nil {
		return nil, err
	}
	defer cr.Close()

	var files []SearchResponseFile
	for {
		var file SearchResponseFile
		err = cr.NextRecord(&file)
		if err != nil {
			break
		}
		files = append(files, file)
	}
	if crErr := cr.GetError(); crErr != nil {
		return nil, fmt.Errorf("reading AQL search results: %w", crErr)
	}

	var arFiles []ArtifactoryFile
	for _, result := range files {
		arFiles = append(arFiles, ArtifactoryFile{
			URI:    path.Join(result.Path, result.Name),
			Folder: false,
			SHA1:   result.SHA1,
		})
	}
	return arFiles, nil
}

func (ars *ArtifactoryReleaseSource) buildArtifactoryServiceManager() (artifactory.ArtifactoryServicesManager, error) {
	rtDetails := auth.NewArtifactoryDetails()
	rtDetails.SetUser(ars.Username)
	rtDetails.SetPassword(ars.Password)
	rtDetails.SetUrl(ars.ArtifactoryHost)
	builder := jfroghttpclient.JfrogClientBuilder()
	builder.SetHttpClient(ars.Client)
	jfHttpClient, err := builder.Build()
	if err != nil {
		return nil, err
	}
	rtDetails.SetClient(jfHttpClient)

	configBuilder := config.NewConfigBuilder()
	configuration, err := configBuilder.SetServiceDetails(rtDetails).SetHttpRetries(3).SetHttpRetryWaitMilliSecs(100).Build()
	if err != nil {
		return nil, err
	}
	am, err := artifactory.New(configuration)
	if err != nil {
		return nil, fmt.Errorf("creating artifactory service manager: %w", err)
	}
	return am, nil
}

type vpnError struct {
	wrapped error
}

func (fe *vpnError) Unwrap() error {
	return fe.wrapped
}

func (fe *vpnError) Error() string {
	return fmt.Sprintf("failed to dial (hint: Are you connected to the corporate vpn?): %s", fe.wrapped)
}

func wrapVPNError(err error) error {
	x := new(net.DNSError)
	if errors.As(err, &x) {
		return &vpnError{wrapped: err}
	}
	// the jfrog api seems to discard type info
	if err != nil && strings.Contains(err.Error(), "lookup :") {
		return &vpnError{wrapped: err}
	}
	return err
}
