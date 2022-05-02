package filesys

import (
	"io/fs"
	"os"
	"path/filepath"
)

type OS struct{}

func WrappingOS() OS {
	return OS{}
}

func (OS) Create(path string) (WFile, error) {
	return os.Create(path)
}

func (OS) Open(path string) (fs.File, error) {
	return os.Open(path)
}

func (OS) Walk(root string, walkFn filepath.WalkFunc) error {
	return filepath.Walk(root, walkFn)
}

func (OS) Remove(path string) error {
	return os.Remove(path)
}
