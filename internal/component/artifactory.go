package component

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"golang.org/x/exp/slices"
	"golang.org/x/net/html"
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
)

const (
	headerChecksumSHA1 = "x-checksum-sha1"
)

type ArtifactoryReleaseSource struct {
	cargo.ReleaseSourceConfig
	Client *http.Client
	logger *log.Logger
	ID     string
}

// NewArtifactoryReleaseSource will provision a new ArtifactoryReleaseSource Project
// from the Kilnfile (ReleaseSourceConfig). If type is incorrect it will PANIC
func NewArtifactoryReleaseSource(c cargo.ReleaseSourceConfig) *ArtifactoryReleaseSource {
	if c.Type != "" && c.Type != ReleaseSourceTypeArtifactory {
		panic(panicMessageWrongReleaseSourceType)
	}

	return &ArtifactoryReleaseSource{
		Client:              http.DefaultClient,
		ReleaseSourceConfig: c,
		ID:                  c.ID,
		logger:              log.New(os.Stderr, "[Artifactory release source] ", log.Default().Flags()),
	}
}

func (ars *ArtifactoryReleaseSource) DownloadRelease(releaseDir string, remoteRelease Lock) (Local, error) {
	downloadURL := ars.ArtifactoryHost + "/artifactory/" + ars.Repo + "/" + remoteRelease.RemotePath
	ars.logger.Printf(logLineDownload, remoteRelease.Name, ReleaseSourceTypeArtifactory, ars.ID)
	resp, err := ars.Client.Get(downloadURL)
	if err != nil {
		return Local{}, wrapVPNError(err)
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

func (ars *ArtifactoryReleaseSource) getFileSHA1(release Lock) (string, error) {
	boshReleaseURL := ars.ArtifactoryHost + "/artifactory/" + ars.Repo + "/" + release.RemotePath
	req, err := http.NewRequest(http.MethodHead, boshReleaseURL, nil)
	if err != nil {
		return "", err
	}
	res, err := ars.Client.Do(req)
	if err != nil {
		return "", wrapVPNError(err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get %s release info from artifactory with error code %d", release.Name, res.StatusCode)
	}
	checksums := res.Header.Values(headerChecksumSHA1)
	if len(checksums) == 0 {
		return "", fmt.Errorf("failed to get %s release info from artifactory with error code %d", release.Name, res.StatusCode)
	}
	return checksums[0], nil
}

func (ars *ArtifactoryReleaseSource) Configuration() cargo.ReleaseSourceConfig {
	return ars.ReleaseSourceConfig
}

// GetMatchedRelease uses the Name and Version and if supported StemcellOS and StemcellVersion
// fields on Requirement to download a specific release.
func (ars *ArtifactoryReleaseSource) GetMatchedRelease(spec Spec) (Lock, error) {
	remotePath, err := ars.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	remoteReleasesDirectoryURL := ars.ArtifactoryHost + "/" + path.Join("artifactory", ars.ReleaseSourceConfig.Repo, remotePath)

	request, err := http.NewRequest(http.MethodHead, remoteReleasesDirectoryURL, nil)
	if err != nil {
		return Lock{}, err
	}

	response, err := ars.Client.Do(request)
	if err != nil {
		return Lock{}, wrapVPNError(err)
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
		SHA1:         response.Header.Get("x-checksum-sha1"),
		RemoteSource: ars.ID,
	}, nil
}

// FindReleaseVersion may use any of the fields on Requirement to return the best matching
// release.
func (ars *ArtifactoryReleaseSource) FindReleaseVersion(spec Spec, _ bool) (Lock, error) {
	remotePath, err := ars.RemotePath(spec)
	if err != nil {
		return Lock{}, err
	}

	fullUrl := fmt.Sprintf("%s/%s/%s/%s/", ars.ArtifactoryHost, "artifactory", ars.Repo, path.Dir(remotePath))

	request, err := http.NewRequest(http.MethodGet, fullUrl, nil)
	if err != nil {
		return Lock{}, err
	}
	request.SetBasicAuth(ars.Username, ars.Password)

	response, err := ars.Client.Do(request)
	if err != nil {
		return Lock{}, wrapVPNError(err)
	}
	defer closeAndIgnoreError(response.Body)

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Lock{}, ErrNotFound
	default:
		return Lock{}, fmt.Errorf("unexpected http status: %s", http.StatusText(response.StatusCode))
	}

	document, err := html.Parse(response.Body)
	if err != nil {
		return Lock{}, fmt.Errorf("failed to parse html response body: %w", err)
	}

	boshReleaseVersionConstraint, err := spec.VersionConstraints()
	if err != nil {
		return Lock{}, fmt.Errorf("failed to parse version constraint: %w", err)
	}

	var locks []Lock
	for _, linkElement := range cascadia.QueryAll(document, cascadia.MustCompile(`a[href]`)) {
		hrefStr := getAttribute(linkElement, "href")
		if hrefStr == "" || strings.HasSuffix(hrefStr, "/") {
			continue
		}
		u, err := url.Parse(hrefStr)
		if err != nil {
			continue
		}
		if path.Ext(u.Path) != ".tgz" {
			continue
		}
		lock, err := lockVersionsFromPath(ars.PathTemplate, path.Base(u.Path))
		if err != nil {
			continue
		}
		v, err := lock.ParseVersion()
		if err != nil {
			continue
		}
		if !boshReleaseVersionConstraint.Check(v) {
			continue
		}
		lock.Name = spec.Name
		lock.StemcellOS = spec.StemcellOS
		lock.StemcellVersion = spec.StemcellVersion
		lock.RemotePath = path.Join(path.Dir(remotePath), path.Base(u.Path))
		lock.RemoteSource = cargo.ReleaseSourceID(ars.ReleaseSourceConfig)
		locks = append(locks, lock)
	}
	if len(locks) == 0 {
		return Lock{}, fmt.Errorf("no locks found for path: %s", remotePath)
	}
	slices.SortFunc(locks, func(a, b Lock) bool {
		av, _ := a.ParseVersion()
		bv, _ := b.ParseVersion()
		return av.LessThan(bv)
	})
	foundRelease := locks[len(locks)-1]
	foundRelease.SHA1, err = ars.getFileSHA1(foundRelease)
	if err != nil {
		return Lock{}, err
	}
	return foundRelease, nil
}

func lockVersionsFromPath(pathTemplateString, fileName string) (Lock, error) {
	fileNameTemplate := pathTemplateString
	lastSlashIndex := strings.LastIndexByte(pathTemplateString, '/')
	if lastSlashIndex >= 0 {
		fileNameTemplate = pathTemplateString[lastSlashIndex+1:]
	}
	hasBOSHReleaseVersion, err := regexp.MatchString(`\{\{.*\.Version.*}}`, fileNameTemplate)
	if err != nil {
		return Lock{}, err
	}
	if !hasBOSHReleaseVersion {
		return Lock{}, fmt.Errorf("path template does not specify .Version")
	}
	hasStemcellVersion, err := regexp.MatchString(`\{\{.*\.StemcellVersion.*}}`, fileNameTemplate)
	if err != nil {
		return Lock{}, err
	}

	semverPattern := regexp.MustCompile(`([-v])(?P<version>\d+(.\d+)(.\d+)?)`)
	versionMatchIndex := semverPattern.SubexpIndex("version")
	versionMatches := semverPattern.FindAllStringSubmatch(filepath.Base(fileName), -1)

	if hasStemcellVersion && len(versionMatches) < 2 {
		return Lock{}, fmt.Errorf("path template (base file name) contains .StemcellVersion and .Version but provided file name does not have two versions")
	}

	var stemcellVersion string
	if len(versionMatches) > 1 {
		stemcellVersion = versionMatches[1][versionMatchIndex]
	}

	return Lock{
		Version:         versionMatches[0][versionMatchIndex],
		StemcellVersion: stemcellVersion,
	}, nil
}

func (ars *ArtifactoryReleaseSource) UploadRelease(spec Spec, file io.Reader) (Lock, error) {
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

	response, err := ars.Client.Do(request)
	if err != nil {
		return Lock{}, wrapVPNError(err)
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

func (ars *ArtifactoryReleaseSource) RemotePath(spec Spec) (string, error) {
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
	return err
}

func getAttribute(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}
