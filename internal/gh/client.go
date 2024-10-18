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
Creates a GitHub client based on the host.
If host = GitHub Enterprise Host, uses githubEnterpriseAccessToken
Else it assumes host as GitHub.com and uses githubAccessToken
*/

func Client(ctx context.Context, host, githubAccessToken string, githubEnterpriseAccessToken string) (*github.Client, error) {
	if host != "" && strings.HasSuffix(host, "broadcom.net") {
		if githubEnterpriseAccessToken == "" {
			return nil, errors.New("github enterprise access token is absent")
		}
		client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubEnterpriseAccessToken}))
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
