package cargo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleaseSourceID_zero(t *testing.T) {
	assert.Equal(t, "", ReleaseSourceID(ReleaseSourceConfig{}))
}

func TestReleaseSourceID_id(t *testing.T) {
	for _, tt := range []struct {
		Type string
	}{
		{Type: ReleaseSourceTypeArtifactory},
		{Type: ReleaseSourceTypeBOSHIO},
		{Type: ReleaseSourceTypeGithub},
		{Type: ReleaseSourceTypeS3},
	} {
		t.Run(tt.Type, func(t *testing.T) {
			configuration := releaseSourceWithAllFieldsSet("x")
			configuration.Type = tt.Type
			configuration.ID = "identifier"
			id := ReleaseSourceID(configuration)
			assert.Equal(t, "identifier", id)
		})
	}
}

func TestReleaseSourceID_artifactory(t *testing.T) {
	configuration := releaseSourceWithAllFieldsSet("x")
	configuration.Type = ReleaseSourceTypeArtifactory
	configuration.ID = ""
	defaultReleaseSourceIDIsType(t, configuration)
}

func TestReleaseSourceID_bosh(t *testing.T) {
	configuration := releaseSourceWithAllFieldsSet("x")
	configuration.Type = ReleaseSourceTypeBOSHIO
	configuration.ID = ""
	defaultReleaseSourceIDIsType(t, configuration)
}

func TestReleaseSourceID_github(t *testing.T) {
	configuration := releaseSourceWithAllFieldsSet("x")
	configuration.Type = ReleaseSourceTypeGithub
	configuration.ID = ""
	configuration.Org = "identifier"
	assert.Equal(t, "identifier", ReleaseSourceID(configuration))
}

func TestReleaseSourceID_s3(t *testing.T) {
	configuration := releaseSourceWithAllFieldsSet("x")
	configuration.Type = ReleaseSourceTypeS3
	configuration.ID = ""
	configuration.Bucket = "identifier"
	assert.Equal(t, "identifier", ReleaseSourceID(configuration))
}

func releaseSourceWithAllFieldsSet(value string) ReleaseSourceConfig {
	return ReleaseSourceConfig{
		Type:            value,
		ID:              value,
		Publishable:     true,
		Bucket:          value,
		Region:          value,
		AccessKeyId:     value,
		SecretAccessKey: value,
		PathTemplate:    value,
		Endpoint:        value,
		Org:             value,
		GithubToken:     value,
		Repo:            value,
		ArtifactoryHost: value,
		Username:        value,
		Password:        value,
	}
}

func defaultReleaseSourceIDIsType(t *testing.T, configuration ReleaseSourceConfig) {
	t.Helper()
	id := ReleaseSourceID(configuration)
	assert.Equal(t, configuration.Type, id)
}
