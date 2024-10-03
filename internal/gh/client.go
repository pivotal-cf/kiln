package gh

import (
	"context"

	"github.com/google/go-github/v65/github"
	"golang.org/x/oauth2"
)

func Client(ctx context.Context, accessToken string) *github.Client {
	return github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})))
}
