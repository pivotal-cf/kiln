package component

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	auth "github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
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

	"github.com/Masterminds/semver/v3"
	"github.com/jfrog/jfrog-client-go/artifactory"

	"github.com/pivotal-cf/kiln/pkg/cargo"
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

	var files []SearchResponseFile
	for {
		var file SearchResponseFile
		err = cr.NextRecord(&file)
		if err != nil {
			break
		}
		files = append(files, file)
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

func (ars *ArtifactoryReleaseSource) getFileInfo(filepath string) (*utils.FileInfo, error) {
	am, err := ars.buildArtifactoryServiceManager()
	if err != nil {
		return nil, err
	}
	fullPath, err := url.JoinPath(ars.Repo, filepath)
	if err != nil {
		return nil, err
	}
	fi, err := am.FileInfo(fullPath)
	return fi, err
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
	am, _ := artifactory.New(configuration)
	return am, nil
}

type ArtifactoryReleaseSource struct {
	cargo.ReleaseSourceConfig
	Client *http.Client
	logger *log.Logger
	ID     string
}

type ArtifactoryFileMetadata struct {
	Checksums struct {
		Sha1   string `json:"sha1"`
		Sha256 string `json:"sha256"`
		MD5    string `json:"md5"`
	} `json:"checksums"`
}

type ArtifactoryFolderInfo struct {
	Children []struct {
		URI    string `json:"uri"`
		Folder bool   `json:"folder"`
	} `json:"children"`
	Path string `json:"path"`
}

type ArtifactoryFile struct {
	URI    string `json:"uri"`
	Folder bool   `json:"folder"`
	SHA1   string
}
type ArtifactoryListInfo struct {
	Files []ArtifactoryFile `json:"files"`
}

// https://github.com/Masterminds/semver/blob/1558ca3488226e3490894a145e831ad58a5ff958/version.go#L44
const (
	semverRegex = `v?(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:\.(0|[1-9]\d*))?` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?`

	reReleaseVersionGroup  = "bosh_version"
	reStemcellVersionGroup = "bosh_stemcell_version"
)

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

func (ars *ArtifactoryReleaseSource) getFileSHA1(release cargo.BOSHReleaseTarballLock) (string, error) {
	info, err := ars.getFileInfo(release.RemotePath)
	if err != nil {
		return "", err
	}
	return info.Checksums.Sha1, nil
}

func (ars *ArtifactoryReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return ars.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (ars *ArtifactoryReleaseSource) GetMatchedRelease(spec cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
	matchedRelease, err := ars.findReleaseVersion(spec, spec)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
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
	artifactoryFiles, err := ars.searchAql(remoteSearchPath)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, wrapVPNError(err)
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
			newVersion, _ := semver.NewVersion(version)
			check := constraint.Check(newVersion)
			if !check {
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
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
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

func (ars *ArtifactoryReleaseSource) UploadRelease(spec cargo.BOSHReleaseTarballSpecification, file io.Reader) (cargo.BOSHReleaseTarballLock, error) {
	remotePath, err := ars.RemotePath(spec)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}

	ars.logger.Printf("uploading release %q to %s at %q...\n", spec.Name, ars.ID, remotePath)

	fullUrl := ars.ArtifactoryHost + "/artifactory/" + ars.Repo + "/" + remotePath

	request, err := http.NewRequest(http.MethodPut, fullUrl, file)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, err
	}
	request.SetBasicAuth(ars.Username, ars.Password)
	// TODO: check Sha1/2
	// request.Header.Set("X-Checksum-Sha1", spec.??? )

	response, err := ars.Client.Do(request)
	if err != nil {
		return cargo.BOSHReleaseTarballLock{}, wrapVPNError(err)
	}

	switch response.StatusCode {
	case http.StatusCreated:
	default:
		return cargo.BOSHReleaseTarballLock{}, fmt.Errorf("response contained errror status code: %d %s", response.StatusCode, response.Status)
	}

	return cargo.BOSHReleaseTarballLock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: ars.ReleaseSourceConfig.ID,
	}, nil
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
			Parse(ars.ReleaseSourceConfig.PathTemplate))
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
