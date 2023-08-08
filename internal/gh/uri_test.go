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
		ErrorSubstring string
	}{
		{
			Name:            "valid url",
			URI:             "https://github.com/crhntr/hello-release",
			RepositoryOwner: "crhntr", RepositoryName: "hello-release",
		},
		{
			Name:            "ssh url",
			URI:             "git@github.com:crhntr/hello-release.git",
			RepositoryOwner: "crhntr", RepositoryName: "hello-release",
		},
		{
			Name:           "empty ssh path",
			URI:            "git@github.com:",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "not a valid ssh path",
			URI:            "git@github.com:?invalid_url?",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "missing repo name",
			URI:            "https://github.com/crhntr",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "missing repo owner",
			URI:            "https://github.com//crhntr",
			ErrorSubstring: "path missing expected parts",
		},
		{
			Name:           "invalid URL",
			URI:            "/?bell-character=\x07",
			ErrorSubstring: "invalid control character in URL",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			repoOwner, repoName, err := gh.RepositoryOwnerAndNameFromPath(tt.URI)
			if tt.ErrorSubstring != "" {
				require.ErrorContains(t, err, tt.ErrorSubstring)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.RepositoryOwner, repoOwner)
				assert.Equal(t, tt.RepositoryName, repoName)
			}
		})
	}
}
