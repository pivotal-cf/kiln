package component

import (
	"bytes"
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
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type ArtifactoryReleaseSource struct {
	cargo.ReleaseSourceConfig
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
}

type ArtifactoryFileInfo struct {
	Checksums struct {
		SHA1 string `json:"sha1"`
	} `json:"checksums"`
}

// NewArtifactoryReleaseSource will provision a new ArtifactoryReleaseSource Project
// from the Kilnfile (ReleaseSourceConfig). If type is incorrect it will PANIC
func NewArtifactoryReleaseSource(c cargo.ReleaseSourceConfig) *ArtifactoryReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeArtifactory {
		panic(panicMessageWrongReleaseSourceType)
	}

	// ctx := context.TODO()

	return &ArtifactoryReleaseSource{
		ReleaseSourceConfig: c,
		ID:                  c.ID,
		logger:              log.New(os.Stdout, "[Artifactory release source] ", log.Default().Flags()),
	}
}

func (ars ArtifactoryReleaseSource) DownloadRelease(releaseDir string, remoteRelease Lock) (Local, error) {
	downloadURL := ars.ArtifactoryHost + "/artifactory/" + ars.Repo + "/" + remoteRelease.RemotePath
	ars.logger.Printf(logLineDownload, remoteRelease.Name, ReleaseSourceTypeArtifactory, ars.ID)
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

func (ars ArtifactoryReleaseSource) getFileSHA1(release Lock) (string, error) {
	fullURL := ars.ArtifactoryHost + "/api/storage/" + ars.Repo + "/" + release.RemotePath
	ars.logger.Printf("Getting %s file info from artifactory", release.Name)
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

func (ars ArtifactoryReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return ars.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (ars ArtifactoryReleaseSource) GetMatchedRelease(spec Spec) (Lock, error) {
	remotePath, err := ars.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	fullUrl := fmt.Sprintf("%s/%s/%s/%s", ars.ArtifactoryHost, "api/storage", ars.Repo, remotePath)
	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return Lock{}, err
	}
	request.SetBasicAuth(ars.Username, ars.Password)

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
		RemoteSource: ars.ID,
	}, nil
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (ars ArtifactoryReleaseSource) FindReleaseVersion(spec Spec) (Lock, error) {
	remotePath, err := ars.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	fullUrl := fmt.Sprintf("%s/%s/%s/%s", ars.ArtifactoryHost, "api/storage", ars.Repo, path.Dir(remotePath))

	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return Lock{}, err
	}
	request.SetBasicAuth(ars.Username, ars.Password)

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
					RemoteSource: ars.ReleaseSourceConfig.ID,
				}
			} else {
				foundVersion, _ := semver.NewVersion(foundRelease.Version)
				if newVersion.GreaterThan(foundVersion) {
					foundRelease = Lock{
						Name:         spec.Name,
						Version:      version,
						RemotePath:   remotePathToUpdate,
						RemoteSource: ars.ReleaseSourceConfig.ID,
					}
				}
			}
		}
	}

	if (foundRelease == Lock{}) {
		return Lock{}, ErrNotFound
	}
	var sha1 string
	sha1, err = ars.getFileSHA1(foundRelease)
	if err != nil {
		return Lock{}, err
	}
	foundRelease.SHA1 = sha1
	return foundRelease, nil
}

func (ars ArtifactoryReleaseSource) UploadRelease(spec Spec, file io.Reader) (Lock, error) {
	remotePath, err := ars.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	ars.logger.Printf("uploading release %q to %s at %q...\n", spec.Name, ars.ID, remotePath)

	fullUrl := ars.ArtifactoryHost + "/artifactory/" + ars.Repo + "/" + remotePath

	request, err := http.NewRequest(http.MethodPut, fullUrl, file)
	if err != nil {
		fmt.Println(err)
		return Lock{}, err
	}
	request.SetBasicAuth(ars.Username, ars.Password)
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
		RemoteSource: ars.ReleaseSourceConfig.ID,
	}, nil
}

func (ars ArtifactoryReleaseSource) RemotePath(spec Spec) (string, error) {
	pathBuf := new(bytes.Buffer)

	err := ars.pathTemplate().Execute(pathBuf, spec)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate path_template: %w", err)
	}

	return pathBuf.String(), nil
}

func (ars ArtifactoryReleaseSource) pathTemplate() *template.Template {
	return template.Must(
		template.New("remote-path").
			Funcs(template.FuncMap{"trimSuffix": strings.TrimSuffix}).
			Parse(ars.ReleaseSourceConfig.PathTemplate))
}
