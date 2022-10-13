package pivnet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	ProductionHost = "network.pivotal.io"
)

type stringError string

func (str stringError) Error() string { return string(str) }

const (
	ErrDecodingURLRequest                 = stringError("error while decoding the url")
	ErrCouldNotCreateRequest              = stringError("could not create valid http request")
	ErrProductSlugMustNotBeEmpty          = stringError("product slug must not be empty")
	ErrStemcellMajorVersionMustNotBeEmpty = stringError("stemcell major version must not be empty")
)

// Service wraps requests to network.pivotal.io.
type Service struct {
	// Target defaults to the public deployed endpoint.
	// It can be set to another host, for example the
	// network.pivotal.io's staging host.
	Target string

	// UAAAPIToken should be set with the token for the "UAA API Token Workflow"
	// See: https://network.pivotal.io/docs/api#authentication
	UAAAPIToken string

	// Client allows you to inject an alternate client
	// (for testing per se). When not set, http.DefaultClient is used.
	Client *http.Client
}

func (service *Service) SetToken(token string) {
	service.UAAAPIToken = token
}

// Do sets required headers for requests to network.pivotal.io.
// If service.Client is nil, it uses http.DefaultClient.
func (service *Service) Do(req *http.Request) (*http.Response, error) {
	if service.Target == "" {
		service.Target = ProductionHost
	}
	if service.Client == nil {
		service.Client = http.DefaultClient
	}

	if service.UAAAPIToken != "" {
		var auth strings.Builder
		auth.WriteString("Bearer ")
		auth.WriteString(service.UAAAPIToken)
		req.Header.Set("Authorization", auth.String())
	}

	if val := req.Header.Get("Accept"); val == "" {
		req.Header.Set("Accept", "application/json")
	}
	if val := req.Header.Get("Content-Type"); val == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if val := req.Header.Get("User-Agent"); val == "" {
		req.Header.Set("User-Agent", "kiln")
	}

	req.URL.Host = service.Target
	if req.URL.Scheme == "" {
		req.URL.Scheme = "https"
	}

	return service.Client.Do(req)
}

type Release struct {
	Version string `json:"version"`
	ID      int    `json:"id"`
}

func (service *Service) Releases(productSlug string) ([]Release, error) {
	req, _ := http.NewRequest(http.MethodGet, "/api/v2/products/"+productSlug+"/releases", nil)

	var body struct {
		Releases []Release `json:"releases"`
	}

	res, err := service.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading the network.pivotal.io response body failed: %s", err)
	}

	if err := json.Unmarshal(responseBody, &body); err != nil {
		return nil, fmt.Errorf("json from %s is malformed: %s", req.URL.Host, err)
	}

	return body.Releases, nil
}

func (service *Service) StemcellVersion(slug string, majorStemcellVersion string) (string, error) {
	if slug == "" {
		return "", ErrProductSlugMustNotBeEmpty
	}

	if majorStemcellVersion == "" {
		return "", ErrStemcellMajorVersionMustNotBeEmpty
	}

	locator := url.URL{
		Scheme:   "https",
		Path:     path.Join("/api/v2/products", slug, "releases/latest"),
		RawQuery: fmt.Sprintf("major=%s", majorStemcellVersion),
	}

	getURL, err := url.PathUnescape(locator.String())
	if err != nil {
		return "", ErrDecodingURLRequest
	}

	req, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		return "", ErrCouldNotCreateRequest
	}

	response, err := service.Do(req)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf("could not make pivnet request: endpoint requires authorization (set --pivotal-network-token with UAA token)")
		}
		return "", fmt.Errorf("request was not successful, response had status %s (%d)", response.Status, response.StatusCode)
	}

	responesBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var body struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(responesBody, &body); err != nil {
		return "", err
	}

	version := body.Version

	return version, nil
}
