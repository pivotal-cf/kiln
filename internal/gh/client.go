package gh

import (
	"context"

	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
)

func Client(ctx context.Context, accessToken string) *github.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	return github.NewClient(tokenClient)
}
