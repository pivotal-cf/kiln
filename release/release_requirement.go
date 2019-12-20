package release

import "github.com/pivotal-cf/kiln/internal/cargo"

func newReleaseRequirement(release cargo.Release, stemcell cargo.Stemcell) ReleaseRequirement {
	return ReleaseRequirement{
		Name:            release.Name,
		Version:         release.Version,
		StemcellOS:      stemcell.OS,
		StemcellVersion: stemcell.Version,
	}
}

type ReleaseRequirement struct {
	Name, Version, StemcellOS, StemcellVersion string
}

func (rr ReleaseRequirement) releaseID() ReleaseID {
	return ReleaseID{Name: rr.Name, Version: rr.Version}
}
