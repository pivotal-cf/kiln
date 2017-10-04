package helper

import (
	"io"
	"os"
	"path/filepath"
)

var OpenFile = os.OpenFile

type Filesystem struct{}

func NewFilesystem() Filesystem {
	return Filesystem{}
}

func (f Filesystem) Open(name string) (io.ReadWriteCloser, error) {
	return os.Open(name)
}

func (f Filesystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return filepath.Walk(root, walkFn)
}
