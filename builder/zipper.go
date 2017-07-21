package builder

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Zipper struct {
	writer *zip.Writer
}

func NewZipper() Zipper {
	return Zipper{}
}

func (z *Zipper) SetPath(path string) error {
	tileFile, err := os.Create(path)
	if err != nil {
		return err
	}

	z.writer = zip.NewWriter(tileFile)

	return nil
}

func (z Zipper) Add(path string, file io.Reader) error {
	if z.writer == nil {
		return errors.New("zipper path must be set")
	}

	f, err := z.writer.Create(path)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, file)
	if err != nil {
		return err
	}

	return nil
}

func (z Zipper) Close() error {
	return z.writer.Close()
}

func (z Zipper) CreateFolder(path string) error {
	if z.writer == nil {
		return errors.New("zipper path must be set")
	}

	path = fmt.Sprintf("%s%c", filepath.Clean(path), filepath.Separator)

	_, err := z.writer.Create(path)
	if err != nil {
		return err
	}

	return err
}
