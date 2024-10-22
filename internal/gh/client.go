package gh

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

/* Client

This method doesn't support creating a GitHub Client based on the host and is soon to be deprecated. Kindly use the GitClient
method below
*/

func Client(ctx context.Context, githubAccessToken string) (*github.Client, error) {
	client, err := GitClient(ctx, "", githubAccessToken, githubAccessToken)
	if err != nil {
		return nil, err
	}
	return client, nil
}

/* GitClient

Creates a GitHub client based on the host.
If host = GitHub Enterprise Host, uses githubEnterpriseAccessToken
Else it assumes host as GitHub.com and uses githubAccessToken
*/

func GitClient(ctx context.Context, host, githubAccessToken string, githubEnterpriseAccessToken string) (*github.Client, error) {
	if host != "" && strings.HasSuffix(host, "broadcom.net") {
		if githubEnterpriseAccessToken == "" {
			return nil, errors.New("github enterprise access token is absent")
		}
		client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubEnterpriseAccessToken}))
		if !strings.HasPrefix(host, "http") {
			host = "https://" + host
		}
		return github.NewEnterpriseClient(host, host, client)
	} else if host == "" || host == "github.com" {
		if githubAccessToken == "" {
			return nil, fmt.Errorf("github access token (github.com) is absent")
		}
		client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubAccessToken}))
		return github.NewClient(client), nil
	}
	return nil, errors.New("github host not recognized")
}
