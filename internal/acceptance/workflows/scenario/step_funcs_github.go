package scenario

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pivotal-cf/kiln/internal/gh"
)

func githubRepoHasReleaseWithTag(ctx context.Context, repoOrg, repoName, tag string) error {
	accessToken, err := githubToken(ctx)
	if err != nil {
		return err
	}
	ghAPI, err := gh.Client(ctx, "", accessToken)
	if err != nil {
		return fmt.Errorf("failed to setup github client: %w", err)
	}
	_, response, err := ghAPI.Repositories.GetReleaseByTag(ctx, repoOrg, repoName, tag)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status on response %d", response.StatusCode)
	}
	return nil
}
