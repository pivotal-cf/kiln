package fakes

import "io"

type Filesystem struct {
	OpenCall struct {
		Receives struct {
			Path string
		}
		Returns struct {
			File  io.ReadWriteCloser
			Error error
		}
		Stub func(path string) (io.ReadWriteCloser, error)
	}
}

func (f *Filesystem) Open(path string) (io.ReadWriteCloser, error) {
	f.OpenCall.Receives.Path = path

	if f.OpenCall.Stub != nil {
		return f.OpenCall.Stub(path)
	}

	return f.OpenCall.Returns.File, f.OpenCall.Returns.Error
}
