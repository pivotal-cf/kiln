package fakes

import "io"

type Zipper struct {
	SetPathCall struct {
		CallCount int
		Receives  struct {
			Path string
		}
		Returns struct {
			Error error
		}
	}

	AddCall struct {
		Stub    func(path string, file io.Reader) error
		Calls   []ZipperAddCall
		Returns struct {
			Error error
		}
	}

	CloseCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	CreateFolderCall struct {
		CallCount int
		Receives  struct {
			Path string
		}
		Returns struct {
			Error error
		}
	}
}

type ZipperAddCall struct {
	Path string
	File io.Reader
}

func (z *Zipper) SetPath(path string) error {
	z.SetPathCall.CallCount++
	z.SetPathCall.Receives.Path = path
	return z.SetPathCall.Returns.Error
}

func (z *Zipper) Add(path string, file io.Reader) error {
	if z.AddCall.Stub != nil {
		return z.AddCall.Stub(path, file)
	}

	z.AddCall.Calls = append(z.AddCall.Calls, ZipperAddCall{Path: path, File: file})

	return z.AddCall.Returns.Error
}

func (z *Zipper) Close() error {
	z.CloseCall.CallCount++
	return z.CloseCall.Returns.Error
}

func (z *Zipper) CreateFolder(path string) error {
	z.CreateFolderCall.CallCount++
	z.CreateFolderCall.Receives.Path = path
	return z.CreateFolderCall.Returns.Error
}
