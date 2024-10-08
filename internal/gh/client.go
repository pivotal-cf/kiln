package gh

import (
	"context"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

func Client(ctx context.Context, host, accessToken string) (*github.Client, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
	if host == "" {
		return github.NewClient(client), nil
	}
	return github.NewEnterpriseClient(host, host, client)
}
