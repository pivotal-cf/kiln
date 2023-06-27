package cargo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleaseSourceID(t *testing.T) {
	for _, tt := range []struct {
		Name          string
		ExpectedID    string
		Configuration ReleaseSourceConfig
	}{
		{Name: "zero value", ExpectedID: "", Configuration: ReleaseSourceConfig{}},
		{Name: "unknown type", ExpectedID: "", Configuration: ReleaseSourceConfig{Type: "banana"}},

		{Name: BOSHReleaseTarballSourceTypeArtifactory + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: BOSHReleaseTarballSourceTypeArtifactory}},
		{Name: BOSHReleaseTarballSourceTypeBOSHIO + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: BOSHReleaseTarballSourceTypeBOSHIO}},
		{Name: BOSHReleaseTarballSourceTypeGithub + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: BOSHReleaseTarballSourceTypeGithub}},
		{Name: BOSHReleaseTarballSourceTypeS3 + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: BOSHReleaseTarballSourceTypeS3}},

		{Name: BOSHReleaseTarballSourceTypeArtifactory + " default", ExpectedID: BOSHReleaseTarballSourceTypeArtifactory, Configuration: ReleaseSourceConfig{ID: "", Type: BOSHReleaseTarballSourceTypeArtifactory}},
		{Name: BOSHReleaseTarballSourceTypeBOSHIO + " default", ExpectedID: BOSHReleaseTarballSourceTypeBOSHIO, Configuration: ReleaseSourceConfig{ID: "", Type: BOSHReleaseTarballSourceTypeBOSHIO}},
		{Name: BOSHReleaseTarballSourceTypeGithub + " default", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "", Type: BOSHReleaseTarballSourceTypeGithub, Org: "identifier"}},
		{Name: BOSHReleaseTarballSourceTypeS3 + " default", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "", Type: BOSHReleaseTarballSourceTypeS3, Bucket: "identifier"}},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Equal(t, tt.ExpectedID, BOSHReleaseTarballSourceID(tt.Configuration))
		})
	}
}
