package fetcher

import "fmt"

type CompiledRelease struct {
	ID              ReleaseID
	StemcellOS      string
	StemcellVersion string
	Path            string
}

func (cr CompiledRelease) RemotePath() string {
	return cr.Path
}

func (cr CompiledRelease) StandardizedFilename() string {
	return fmt.Sprintf("%s-%s-%s-%s.tgz", cr.ID.Name, cr.ID.Version, cr.StemcellOS, cr.StemcellVersion)
}

func (cr CompiledRelease) LocalPath() string {
	return cr.Path
}

func (cr CompiledRelease) Satisfies(rr ReleaseRequirement) bool {
	return cr.ID.Name == rr.Name &&
		cr.ID.Version == rr.Version &&
		cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}

func (cr CompiledRelease) ReleaseID() ReleaseID {
	return cr.ID
}

func (cr CompiledRelease) AsLocal(path string) LocalRelease {
	cr.Path = path
	return cr
}
