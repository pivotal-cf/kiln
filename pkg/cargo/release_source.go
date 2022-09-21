package cargo

const (
	// ReleaseSourceTypeBOSHIO is the value of the Type field on cargo.ReleaseSourceConfig
	// for fetching https://bosh.io releases.
	ReleaseSourceTypeBOSHIO = "bosh.io"

	// ReleaseSourceTypeS3 is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on
	ReleaseSourceTypeS3 = "s3"

	// ReleaseSourceTypeGithub is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on GitHub.
	ReleaseSourceTypeGithub = "github"

	// ReleaseSourceTypeArtifactory is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on Artifactory.
	ReleaseSourceTypeArtifactory = "artifactory"
)

func ReleaseSourceID(releaseConfig ReleaseSourceConfig) string {
	if releaseConfig.ID != "" {
		return releaseConfig.ID
	}
	switch releaseConfig.Type {
	case ReleaseSourceTypeBOSHIO:
		return ReleaseSourceTypeBOSHIO
	case ReleaseSourceTypeS3:
		return releaseConfig.Bucket
	case ReleaseSourceTypeGithub:
		return releaseConfig.Org
	case ReleaseSourceTypeArtifactory:
		return ReleaseSourceTypeArtifactory
	default:
		return ""
	}
}
