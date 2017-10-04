package fakes

import (
	"io"
	"path/filepath"
)

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
	WalkCall struct {
		Receives struct {
			Root string
		}
		Returns struct {
			Error error
		}
		Stub func(root string, walkFn filepath.WalkFunc) error
	}
}

func (f *Filesystem) Open(path string) (io.ReadWriteCloser, error) {
	f.OpenCall.Receives.Path = path

	if f.OpenCall.Stub != nil {
		return f.OpenCall.Stub(path)
	}

	return f.OpenCall.Returns.File, f.OpenCall.Returns.Error
}

func (f *Filesystem) Walk(root string, walkFn filepath.WalkFunc) error {
	f.WalkCall.Receives.Root = root

	if f.WalkCall.Stub != nil {
		return f.WalkCall.Stub(root, walkFn)
	}

	return f.WalkCall.Returns.Error
}
