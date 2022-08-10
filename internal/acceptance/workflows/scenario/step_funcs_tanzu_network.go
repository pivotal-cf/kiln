package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func tanzuNetworkHasProductWithVersion(ctx context.Context, slug, version string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://network.tanzu.vmware.com/api/v2/products/%s/releases", slug), nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("request status code is not ok: got %d", res.StatusCode)
	}
	resBuf, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	var responseBody struct {
		Releases []struct {
			Version string `json:"version"`
		} `json:"releases"`
	}
	err = json.Unmarshal(resBuf, &responseBody)
	if err != nil {
		return err
	}
	for _, release := range responseBody.Releases {
		if release.Version == version {
			return nil
		}
	}
	return fmt.Errorf("TanzuNetwork release with version %s not found", version)
}
