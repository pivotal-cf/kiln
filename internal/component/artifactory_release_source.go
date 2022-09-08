package component

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"
)

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
}

type ArtifactoryFileInfo struct {
	Checksums struct {
		SHA1 string `json:"sha1"`
	} `json:"checksums"`
}

type ArtifactoryReleaseSource struct {
	Identifier  string `yaml:"id,omitempty"`
	Publishable bool   `yaml:"publishable,omitempty"`

	ArtifactoryHost string `yaml:"artifactory_host,omitempty"`
	Username        string `yaml:"username,omitempty"`
	Password        string `yaml:"password,omitempty"`
	Repo            string `yaml:"repo,omitempty"`
	PathTemplate    string `yaml:"path_template,omitempty"`
}

func (src *ArtifactoryReleaseSource) ConfigurationErrors() []error {
	var result []error
	if src.PathTemplate == "" {
		result = append(result, fmt.Errorf(`missing required field "path_template" in release source config`))
	}
	return result
}

func (src *ArtifactoryReleaseSource) ID() string {
	if src.Identifier != "" {
		return src.Identifier
	}
	return ReleaseSourceTypeArtifactory
}

func (src *ArtifactoryReleaseSource) IsPublishable() bool { return src.Publishable }
func (src *ArtifactoryReleaseSource) Type() string        { return ReleaseSourceTypeArtifactory }

func (src *ArtifactoryReleaseSource) GetMatchedRelease(_ context.Context, _ *log.Logger, spec Spec) (Lock, error) {
	return src.getMatchedRelease(spec)
}

func (src *ArtifactoryReleaseSource) FindReleaseVersion(_ context.Context, logger *log.Logger, spec Spec) (Lock, error) {
	return src.findReleaseVersion(logger, spec)
}

func (src *ArtifactoryReleaseSource) DownloadRelease(_ context.Context, logger *log.Logger, releasesDir string, remoteRelease Lock) (Local, error) {
	return src.downloadRelease(logger, releasesDir, remoteRelease)
}

func (src *ArtifactoryReleaseSource) downloadRelease(logger *log.Logger, releaseDir string, remoteRelease Lock) (Local, error) {
	downloadURL := src.ArtifactoryHost + "/artifactory/" + src.Repo + "/" + remoteRelease.RemotePath
	logger.Printf(logLineDownload, remoteRelease.Name, ReleaseSourceTypeArtifactory, src.ID())
	resp, err := http.Get(downloadURL)
	if err != nil {
		return Local{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Local{}, fmt.Errorf("failed to download %s release from artifactory with error code %d", remoteRelease.Name, resp.StatusCode)
	}

	if err != nil {
		return Local{}, err
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

func (src *ArtifactoryReleaseSource) getFileSHA1(logger *log.Logger, release Lock) (string, error) {
	fullURL := src.ArtifactoryHost + "/api/storage/" + src.Repo + "/" + release.RemotePath
	logger.Printf("Getting %s file info from artifactory", release.Name)
	resp, err := http.Get(fullURL)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get %s release info from artifactory with error code %d", release.Name, resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var artifactoryFileInfo ArtifactoryFileInfo

	if err := json.Unmarshal(responseBody, &artifactoryFileInfo); err != nil {
		return "", fmt.Errorf("json is malformed: %s", err)
	}

	return artifactoryFileInfo.Checksums.SHA1, nil
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (src *ArtifactoryReleaseSource) getMatchedRelease(spec Spec) (Lock, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	fullUrl := fmt.Sprintf("%s/%s/%s/%s", src.ArtifactoryHost, "api/storage", src.Repo, remotePath)
	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return Lock{}, err
	}
	request.SetBasicAuth(src.Username, src.Password)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return Lock{}, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Lock{}, ErrNotFound
	default:
		return Lock{}, fmt.Errorf("unexpected http status: %s", http.StatusText(response.StatusCode))
	}

	return Lock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: src.ID(),
	}, nil
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (src *ArtifactoryReleaseSource) findReleaseVersion(logger *log.Logger, spec Spec) (Lock, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	fullUrl := fmt.Sprintf("%s/%s/%s/%s", src.ArtifactoryHost, "api/storage", src.Repo, path.Dir(remotePath))

	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return Lock{}, err
	}
	request.SetBasicAuth(src.Username, src.Password)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return Lock{}, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Lock{}, ErrNotFound
	default:
		return Lock{}, fmt.Errorf("unexpected http status: %s", http.StatusText(response.StatusCode))
	}

	var artifactoryFolderInfo ArtifactoryFolderInfo
	var _ *semver.Constraints

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Lock{}, err
	}

	if err := json.Unmarshal(responseBody, &artifactoryFolderInfo); err != nil {
		return Lock{}, fmt.Errorf("json from %s is malformed: %s", request.URL.Host, err)
	}

	semverPattern, err := regexp.Compile(`([-v])\d+(.\d+)*`)
	if err != nil {
		return Lock{}, err
	}

	foundRelease := Lock{}
	constraint, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, err
	}

	for _, releases := range artifactoryFolderInfo.Children {
		if releases.Folder {
			continue
		}
		versions := semverPattern.FindAllString(filepath.Base(releases.URI), -1)
		version := versions[0]
		stemcellVersion := versions[len(versions)-1]
		version = strings.Replace(version, "-", "", -1)
		version = strings.Replace(version, "v", "", -1)
		stemcellVersion = strings.Replace(stemcellVersion, "-", "", -1)
		if len(versions) > 1 && stemcellVersion != spec.StemcellVersion {
			continue
		}
		if version != "" {
			newVersion, _ := semver.NewVersion(version)
			if !constraint.Check(newVersion) {
				continue
			}

			remotePathToUpdate := path.Dir(remotePath) + releases.URI

			if (foundRelease == Lock{}) {
				foundRelease = Lock{
					Name:         spec.Name,
					Version:      version,
					RemotePath:   remotePathToUpdate,
					RemoteSource: src.ID(),
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = Lock{
						Name:         spec.Name,
						Version:      version,
						RemotePath:   remotePathToUpdate,
						RemoteSource: src.ID(),
					}
				}
			}
		}
	}

	if (foundRelease == Lock{}) {
		return Lock{}, ErrNotFound
	}
	var sum string
	sum, err = src.getFileSHA1(logger, foundRelease)
	if err != nil {
		return Lock{}, err
	}
	foundRelease.SHA1 = sum
	return foundRelease, nil
}

func (src *ArtifactoryReleaseSource) UploadRelease(ctx context.Context, logger *log.Logger, spec Spec, file io.Reader) (Lock, error) {
	remotePath, err := src.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	logger.Printf("uploading release %q to %s at %q...\n", spec.Name, src.ID(), remotePath)

	fullUrl := src.ArtifactoryHost + "/artifactory/" + src.Repo + "/" + remotePath

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, fullUrl, file)
	if err != nil {
		fmt.Println(err)
		return Lock{}, err
	}
	request.SetBasicAuth(src.Username, src.Password)
	// TODO: check Sha1/2
	// request.Header.Set("X-Checksum-Sha1", spec.??? )

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fmt.Println(err)
		return Lock{}, err
	}

	switch response.StatusCode {
	case http.StatusCreated:
	default:
		return Lock{}, fmt.Errorf(response.Status)
	}

	return Lock{
		Name:         spec.Name,
		Version:      spec.Version,
		RemotePath:   remotePath,
		RemoteSource: src.ID(),
	}, nil
}

func (src *ArtifactoryReleaseSource) RemotePath(spec Spec) (string, error) {
	pathBuf := new(bytes.Buffer)

	err := src.pathTemplate().Execute(pathBuf, spec)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate path_template: %w", err)
	}

	return pathBuf.String(), nil
}

func (src *ArtifactoryReleaseSource) pathTemplate() *template.Template {
	return template.Must(
		template.New("remote-path").
			Funcs(template.FuncMap{"trimSuffix": strings.TrimSuffix}).
			Parse(src.PathTemplate))
}
