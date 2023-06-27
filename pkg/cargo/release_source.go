package cargo

const (
	// BOSHReleaseTarballSourceTypeBOSHIO is the value of the Type field on cargo.ReleaseSourceConfig
	// for fetching https://bosh.io releases.
	BOSHReleaseTarballSourceTypeBOSHIO = "bosh.io"

	// BOSHReleaseTarballSourceTypeS3 is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on
	BOSHReleaseTarballSourceTypeS3 = "s3"

	// BOSHReleaseTarballSourceTypeGithub is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on GitHub.
	BOSHReleaseTarballSourceTypeGithub = "github"

	// BOSHReleaseTarballSourceTypeArtifactory is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on Artifactory.
	BOSHReleaseTarballSourceTypeArtifactory = "artifactory"
)

func BOSHReleaseTarballSourceID(releaseConfig ReleaseSourceConfig) string {
	if releaseConfig.ID != "" {
		return releaseConfig.ID
	}
	switch releaseConfig.Type {
	case BOSHReleaseTarballSourceTypeBOSHIO:
		return BOSHReleaseTarballSourceTypeBOSHIO
	case BOSHReleaseTarballSourceTypeS3:
		return releaseConfig.Bucket
	case BOSHReleaseTarballSourceTypeGithub:
		return releaseConfig.Org
	case BOSHReleaseTarballSourceTypeArtifactory:
		return BOSHReleaseTarballSourceTypeArtifactory
	default:
		return ""
	}
}
