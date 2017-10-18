package fakes

import "github.com/pivotal-cf/kiln/builder"

type MetadataReader struct {
	ReadCall struct {
		Receives struct {
			Path    string
			Version string
		}
		Returns struct {
			Metadata builder.Metadata
			Error    error
		}
	}
}

func (r *MetadataReader) Read(path, version string) (builder.Metadata, error) {
	r.ReadCall.Receives.Path = path

	return r.ReadCall.Returns.Metadata, r.ReadCall.Returns.Error
}
