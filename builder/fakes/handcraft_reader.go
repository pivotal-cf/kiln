package fakes

import "github.com/pivotal-cf/kiln/builder"

type HandcraftReader struct {
	ReadCall struct {
		Receives struct {
			Path    string
			Version string
		}
		Returns struct {
			Handcraft builder.Handcraft
			Error     error
		}
	}
}

func (r *HandcraftReader) Read(path, version string) (builder.Handcraft, error) {
	r.ReadCall.Receives.Path = path

	return r.ReadCall.Returns.Handcraft, r.ReadCall.Returns.Error
}
