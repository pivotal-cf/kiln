package helper

import (
	"io"
	"os"
)

var OpenFile = os.OpenFile

type Filesystem struct{}

func NewFilesystem() Filesystem {
	return Filesystem{}
}

func (f Filesystem) Open(name string) (io.ReadWriteCloser, error) {
	return os.Open(name)
}
