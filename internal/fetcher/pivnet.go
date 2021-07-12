package fetcher

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	ErrCouldNotCreateRequest     = stringError("could not create valid http request")
	ErrProductSlugMustNotBeEmpty = stringError("product slug must not be empty")
)

// Pivnet handles kiln specific request to network.pivotal.io
type Pivnet struct {

	// UAAAPIToken should be set with the token for the "UAA API Token Workflow"
	// See: https://network.pivotal.io/docs/api#authentication
	UAAAPIToken string

	// Client allows you to inject an alternate client
	// (for testing per say). When not set, http.DefaultClient is used.
	Client *http.Client
}

func (pivnet *Pivnet) SetToken(token string) {
	pivnet.UAAAPIToken = token
}

func (pivnet *Pivnet) Versions(slug string) ([]string, error) {
	if slug == "" {
		return nil, ErrProductSlugMustNotBeEmpty
	}
	locator := url.URL{
		Scheme: "https",
		Host:   "network.pivotal.io",
		Path:   path.Join("/api/v2/products", string(slug), "releases"),
	}

	req, err := http.NewRequest(http.MethodGet, locator.String(), nil)
	if err != nil {
		return nil, ErrCouldNotCreateRequest
	}

	response, err := pivnet.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("could not make pivnet request: endpoint requires authorization (set --pivotal-network-token with UAA token)")
		}
		return nil, fmt.Errorf("request was not successful, response had status %s (%d)", response.Status, response.StatusCode)
	}

	responesBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var body struct {
		Releases []struct {
			Version string `json:"version"`
		} `json:"releases"`
	}

	if err := json.Unmarshal(responesBody, &body); err != nil {
		return nil, err
	}

	var versions []string

	for _, rel := range body.Releases {
		versions = append(versions, rel.Version)
	}

	return versions, nil
}

// Do sets required headers for requests to network.pivotal.io.
// If pivnet.Client is nil, it uses http.DefaultClient.
func (pivnet Pivnet) Do(req *http.Request) (*http.Response, error) {
	if pivnet.Client == nil {
		pivnet.Client = http.DefaultClient
	}

	if pivnet.UAAAPIToken != "" {
		var auth strings.Builder
		auth.WriteString("Bearer ")
		auth.WriteString(pivnet.UAAAPIToken)
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

	return pivnet.Client.Do(req)
}
