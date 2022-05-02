package filesys

import (
	"io"
	"io/fs"
	"path/filepath"
)

//counterfeiter:generate -o ./fakes/filesystem.go --fake-name Interface . Interface
type Interface interface {
	fs.FS

	Create(path string) (WFile, error)
	Walk(root string, walkFn filepath.WalkFunc) error
	Remove(path string) error
}

//counterfeiter:generate -o ./fakes/file.go --fake-name WFile . WFile

type WFile interface {
	fs.File
	io.Writer
}
