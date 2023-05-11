package gh

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
)

func Client(ctx context.Context, accessToken string) *github.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	return github.NewClient(tokenClient)
}

func HTTPClient(ctx context.Context, accessToken string) (*http.Client, error) {
	accessToken = strings.TrimSpace(accessToken)
	if strings.HasPrefix(accessToken, "$(") && strings.HasSuffix(accessToken, ")") {
		var err error
		accessToken, err = gitHubToken(ctx)
		if err != nil {
			return nil, err
		}
	}
	if accessToken == "" {
		return http.DefaultClient, nil
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	return tokenClient, nil
}

func gitHubToken(ctx context.Context) (string, error) {
	ghToken, found := os.LookupEnv("GITHUB_TOKEN")
	if found {
		return ghToken, nil
	}
	ghToken, found = githubTokenFromCLI(ctx)
	if !found {
		return "", fmt.Errorf("failed to configure GitHub API client: the token was not found in GITHUB_TOKEN environment vairable nor from gh auth --show-token CLI command")
	}
	return ghToken, nil
}

func githubTokenFromCLI(ctx context.Context) (string, bool) {
	_, err := exec.LookPath("gh")
	if err != nil {
		return "", false
	}
	var stderr, stdout bytes.Buffer
	ghShowToken := exec.CommandContext(ctx, "gh", "auth", "status", "--show-token")
	ghShowToken.Stderr = &stderr
	ghShowToken.Stdout = &stdout
	if err := ghShowToken.Run(); err != nil {
		return "", false
	}
	exp := regexp.MustCompile(`(?m)Token: (?P<token>gh.+)$`)
	matchIndex := exp.SubexpIndex("token")
	matches := exp.FindStringSubmatch(stderr.String())
	if len(matches) == 0 {
		return "", false
	}
	tok := strings.TrimSpace(matches[matchIndex])
	return tok, true
}
