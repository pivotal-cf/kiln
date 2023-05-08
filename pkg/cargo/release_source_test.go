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

		{Name: ReleaseSourceTypeArtifactory + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: ReleaseSourceTypeArtifactory}},
		{Name: ReleaseSourceTypeBOSHIO + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: ReleaseSourceTypeBOSHIO}},
		{Name: ReleaseSourceTypeGithub + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: ReleaseSourceTypeGithub}},
		{Name: ReleaseSourceTypeS3 + " with ID set", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "identifier", Type: ReleaseSourceTypeS3}},

		{Name: ReleaseSourceTypeArtifactory + " default", ExpectedID: ReleaseSourceTypeArtifactory, Configuration: ReleaseSourceConfig{ID: "", Type: ReleaseSourceTypeArtifactory}},
		{Name: ReleaseSourceTypeBOSHIO + " default", ExpectedID: ReleaseSourceTypeBOSHIO, Configuration: ReleaseSourceConfig{ID: "", Type: ReleaseSourceTypeBOSHIO}},
		{Name: ReleaseSourceTypeGithub + " default", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "", Type: ReleaseSourceTypeGithub, Org: "identifier"}},
		{Name: ReleaseSourceTypeS3 + " default", ExpectedID: "identifier", Configuration: ReleaseSourceConfig{ID: "", Type: ReleaseSourceTypeS3, Bucket: "identifier"}},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Equal(t, tt.ExpectedID, ReleaseSourceID(tt.Configuration))
		})
	}
}
