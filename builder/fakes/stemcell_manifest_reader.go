package fakes

import "github.com/pivotal-cf/kiln/builder"

type StemcellManifestReader struct {
	ReadCall struct {
		Returns struct {
			StemcellManifest builder.StemcellManifest
			Error            error
		}
		Receives struct {
			Path string
		}
	}
}

func (s *StemcellManifestReader) Read(path string) (builder.StemcellManifest, error) {
	s.ReadCall.Receives.Path = path
	return s.ReadCall.Returns.StemcellManifest, s.ReadCall.Returns.Error
}
