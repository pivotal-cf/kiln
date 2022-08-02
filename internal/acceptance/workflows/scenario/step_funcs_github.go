package scenario

import (
	"context"
	"fmt"
	"github.com/pivotal-cf/kiln/internal/gh"
	"net/http"
)

func githubRepoHasReleaseWithTag(ctx context.Context, repoOrg, repoName, tag string) error {
	accessToken, err := githubToken(ctx)
	if err != nil {
		return err
	}
	ghAPI := gh.Client(ctx, accessToken)
	_, response, err := ghAPI.Repositories.GetReleaseByTag(ctx, repoOrg, repoName, tag)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status on response %d", response.StatusCode)
	}
	return nil
}
