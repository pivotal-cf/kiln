package fakes

import (
	"bytes"
	"io"
	"io/ioutil"
)

type ReadCloser struct {
	RC io.ReadCloser

	ReadCall struct {
		Returns struct {
			Err error
		}
	}

	CloseCall struct {
		Count   int
		Returns struct {
			Err error
		}
	}
}

func NewReadCloser(buf string) *ReadCloser {
	return &ReadCloser{
		RC: ioutil.NopCloser(bytes.NewBufferString(buf)),
	}
}

func (mock *ReadCloser) Read(buf []byte) (int, error) {
	if mock.ReadCall.Returns.Err != nil {
		return 0, mock.ReadCall.Returns.Err
	}
	if mock.RC == nil {
		mock.RC = ioutil.NopCloser(bytes.NewBufferString(""))
	}
	return mock.RC.Read(buf)
}

func (mock *ReadCloser) Close() error {
	mock.CloseCall.Count++
	if mock.CloseCall.Returns.Err != nil {
		return mock.CloseCall.Returns.Err
	}
	if mock.RC == nil {
		mock.RC = ioutil.NopCloser(bytes.NewBufferString(""))
	}
	return mock.RC.Close()
}
