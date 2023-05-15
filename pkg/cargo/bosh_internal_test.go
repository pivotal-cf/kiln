package cargo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
	"io"
	"testing"
	"testing/fstest"
	"testing/iotest"
	"time"
)

func Test_internal_BOSHReleaseManifestAndLicense(t *testing.T) {

}

func Test_fileChecksum(t *testing.T) {
	dir := fstest.MapFS{}
	err := fileChecksum(dir, "banana.tgz", sha1.New())
	assert.Error(t, err)
}

func Test_readTarballFiles(t *testing.T) {
	t.Run("read fails", func(t *testing.T) {
		r := iotest.ErrReader(errors.New("lemon"))
		_, err := readTarballFiles(r)
		assert.Error(t, err)
	})
	t.Run("it skips files", func(t *testing.T) {
		tarballBuffer := makeTGZ(t, func(t *testing.T, tw *tar.Writer) {
			addRegularFile(t, tw, "banana.txt", "Hello, world!", len("Hello, world!"))
			addRegularFile(t, tw, "lemon", "Hello, world!", len("Hello, world!"))
		})
		files, err := readTarballFiles(tarballBuffer, "lemon")
		assert.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, []string{"lemon"}, maps.Keys(files))
	})
	t.Run("force read header error", func(t *testing.T) {
		tarballBuffer := makeTGZ(t, func(t *testing.T, tw *tar.Writer) {
			addRegularFile(t, tw, "banana.txt", "Hello!", len("Hello!")+1000)
		})
		_, err := readTarballFiles(tarballBuffer, "lemon")
		assert.Error(t, err)
	})
}

func addRegularFile(t *testing.T, tarball *tar.Writer, fileName, fileContent string, fileSize int) {
	err := tarball.WriteHeader(&tar.Header{
		Name:       fileName,
		Typeflag:   tar.TypeReg,
		Mode:       0666,
		Size:       int64(fileSize),
		ModTime:    time.Unix(783691200, 0),
		ChangeTime: time.Unix(783691200, 0),
		AccessTime: time.Unix(783691200, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(tarball, fileContent)
	if err != nil {
		t.Fatal(err)
	}
}

func makeTGZ(t *testing.T, fns ...func(t *testing.T, tarball *tar.Writer)) *bytes.Buffer {
	t.Helper()
	r := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(r)
	defer closeAndIgnoreError(gzipWriter)
	tarball := tar.NewWriter(gzipWriter)
	defer closeAndIgnoreError(tarball)
	for _, fn := range fns {
		fn(t, tarball)
	}
	return r
}
