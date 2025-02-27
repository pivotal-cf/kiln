package gh_test

import (
	"testing"

	"github.com/pivotal-cf/kiln/internal/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RepositoryOwnerAndNameFromPath(t *testing.T) {
	for _, tt := range []struct {
		Name,
		URI,
		RepositoryOwner, RepositoryName,
		RepositoryHost,
		ErrorSubstring string
	}{
		{
			Name:            "valid url",
			URI:             "https://github.com/releen/hello-release",
			RepositoryOwner: "releen", RepositoryName: "hello-release", RepositoryHost: "github.com",
		},
		{
			Name:            "ssh url",
			URI:             "git@github.com:releen/hello-release.git",
			RepositoryOwner: "releen", RepositoryName: "hello-release", RepositoryHost: "github.com",
		},
		{
			Name:           "empty ssh path",
			URI:            "git@github.com:",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:            "github enterprise",
			URI:             "git@example.com:x/y.git",
			RepositoryOwner: "x", RepositoryName: "y", RepositoryHost: "example.com",
		},
		{
			Name:           "not a valid ssh path",
			URI:            "git@github.com:?invalid_url?",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "missing repo name",
			URI:            "https://github.com/releen",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "missing repo owner",
			URI:            "https://github.com//releen",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "invalid URL",
			URI:            "/?bell-character=\x07",
			ErrorSubstring: "invalid control character in URL",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			repoHost, repoOwner, repoName, err := gh.RepositoryHostOwnerAndNameFromPath(tt.URI)
			if tt.ErrorSubstring != "" {
				require.ErrorContains(t, err, tt.ErrorSubstring)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.RepositoryOwner, repoOwner)
				assert.Equal(t, tt.RepositoryName, repoName)
				assert.Equal(t, tt.RepositoryHost, repoHost)
			}

			repoOwner, repoName, err = gh.RepositoryOwnerAndNameFromPath(tt.URI)
			if tt.ErrorSubstring != "" {
				require.ErrorContains(t, err, tt.ErrorSubstring)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.RepositoryOwner, repoOwner)
				assert.Equal(t, tt.RepositoryName, repoName)
				assert.Equal(t, tt.RepositoryHost, repoHost)
			}
		})
	}
}
