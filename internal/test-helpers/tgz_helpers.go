package test_helpers

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"gopkg.in/src-d/go-billy.v4"
	"io"
	"os"
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

func WriteReleaseTarball(path, name, version string, fs billy.Filesystem) string {
	f, err := fs.Create(path)
	Expect(err).NotTo(HaveOccurred())

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	releaseManifest := `
name: ` + name + `
version: ` + version + `
`
	manifestReader := strings.NewReader(releaseManifest)

	header := &tar.Header{
		Name:    "release.MF",
		Size:    manifestReader.Size(),
		Mode:    int64(os.O_RDONLY),
		ModTime: time.Now(),
	}
	Expect(tw.WriteHeader(header)).To(Succeed())

	_, err = io.Copy(tw, manifestReader)
	Expect(err).NotTo(HaveOccurred())

	Expect(tw.Close()).To(Succeed())
	Expect(gw.Close()).To(Succeed())
	Expect(f.Close()).To(Succeed())

	tarball, err := fs.Open(path)
	Expect(err).NotTo(HaveOccurred())
	defer tarball.Close()

	hash := sha1.New()
	_, err = io.Copy(hash, tarball)
	Expect(err).NotTo(HaveOccurred())

	return fmt.Sprintf("%x", hash.Sum(nil))
}
