package release

import "github.com/pivotal-cf/kiln/internal/cargo"

func newRequirement(release cargo.ReleaseLock, stemcell cargo.Stemcell) Requirement {
	return Requirement{
		Name:            release.Name,
		Version:         release.Version,
		StemcellOS:      stemcell.OS,
		StemcellVersion: stemcell.Version,
	}
}

type Requirement struct {
	Name, Version, StemcellOS, StemcellVersion string
}

func (rr Requirement) releaseID() ID {
	return ID{Name: rr.Name, Version: rr.Version}
}
