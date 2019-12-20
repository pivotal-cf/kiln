package fetcher

import "fmt"

type CompiledRelease struct {
	ID              ReleaseID
	StemcellOS      string
	StemcellVersion string
	localPath       string
	remotePath      string
}

func NewCompiledRelease(id ReleaseID, stemcellOS, stemcellVersion, localPath, remotePath string) CompiledRelease {
	return CompiledRelease{
		ID:              id,
		StemcellOS:      stemcellOS,
		StemcellVersion: stemcellVersion,
		localPath:       localPath,
		remotePath:      remotePath,
	}
}

func (cr CompiledRelease) RemotePath() string {
	return cr.remotePath
}

func (cr CompiledRelease) StandardizedFilename() string {
	return fmt.Sprintf("%s-%s-%s-%s.tgz", cr.ID.Name, cr.ID.Version, cr.StemcellOS, cr.StemcellVersion)
}

func (cr CompiledRelease) LocalPath() string {
	return cr.localPath
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
	cr.localPath = path
	return cr
}
