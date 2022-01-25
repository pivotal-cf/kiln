package component

import (
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"io"
	"testing"
)

type ArtifactoryReleaseSource struct {
	Config cargo.ReleaseSourceConfig
}

func (a ArtifactoryReleaseSource) Configuration() cargo.ReleaseSourceConfig { return a.Config }

func (a ArtifactoryReleaseSource) GetMatchedRelease(spec Spec) (Lock, error) {
	/*
		// these fields need to be set on Lock
		Name
		SHA1
		Version
		StemcellOS
		StemcellVersion
	*/
	//TODO implement me
	panic("implement me")
}

func (a ArtifactoryReleaseSource) FindReleaseVersion(spec Spec) (Lock, error) {
	/*
		// these fields need to be set on Lock
		Name
		SHA1
		Version
		StemcellOS
		StemcellVersion
	*/
	//TODO implement me
	panic("implement me")
}

func (a ArtifactoryReleaseSource) DownloadRelease(releasesDir string, remoteRelease Lock) (Local, error) {
	panic("implement me")
	//TODO implement me
}

func (a ArtifactoryReleaseSource) UploadRelease(spec Spec, file io.Reader) (Lock, error) {
	//TODO implement me
	panic("implement me")
}

func TestArtifactoryReleaseSource(t *testing.T) {

}
