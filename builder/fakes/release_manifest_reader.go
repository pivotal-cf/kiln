package fakes

import "github.com/pivotal-cf/kiln/builder"

type ReleaseManifestReader struct {
	ReadCall struct {
		Stub    func(releaseTarball string) (builder.ReleaseManifest, error)
		Returns struct {
			ReleaseManifest builder.ReleaseManifest
			Error           error
		}
	}
}

func (r ReleaseManifestReader) Read(releaseTarball string) (builder.ReleaseManifest, error) {
	if r.ReadCall.Stub != nil {
		return r.ReadCall.Stub(releaseTarball)
	}

	return r.ReadCall.Returns.ReleaseManifest, r.ReadCall.Returns.Error
}
