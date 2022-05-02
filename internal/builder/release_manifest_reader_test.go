package builder_test

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-billy/v5/osfs"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/builder"
)

func createReleaseTarball(releaseMetadata string) (fs.File, string, error) {
	tarball, err := ioutil.TempFile("", "kiln")
	if err != nil {
		return nil, "", err
	}

	gw := gzip.NewWriter(tarball)
	tw := tar.NewWriter(gw)

	releaseManifest := bytes.NewBuffer([]byte(releaseMetadata))

	header := &tar.Header{
		Name:    "./release.MF",
		Size:    int64(releaseManifest.Len()),
		Mode:    int64(0644),
		ModTime: time.Now(),
	}

	err = tw.WriteHeader(header)
	if err != nil {
		return nil, "", err
	}

	_, err = io.Copy(tw, releaseManifest)
	if err != nil {
		return nil, "", err
	}

	err = tw.Close()
	if err != nil {
		return nil, "", err
	}

	err = gw.Close()
	if err != nil {
		return nil, "", err
	}

	err = tarball.Close()
	if err != nil {
		return nil, "", err
	}

	var file *os.File
	file, err = os.Open(tarball.Name())
	if err != nil {
		return nil, "", err
	}

	hash := sha1.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, "", err
	}

	releaseSHA1 := fmt.Sprintf("%x", hash.Sum(nil))

	err = file.Close()
	if err != nil {
		return nil, "", err
	}

	return tarball, releaseSHA1, nil
}

var _ = Describe("ReleaseManifestReader", func() {
	var (
		reader      builder.ReleaseManifestReader
		releaseSHA1 string
		tarball     fs.File
		err         error
		tarballName string
	)

	BeforeEach(func() {
		reader = builder.NewReleaseManifestReader(osfs.New(""))
		tarball, releaseSHA1, err = createReleaseTarball(`
name: release
version: 1.2.3
compiled_packages:
- name: some-package
  stemcell: ubuntu-xenial/170.25
`)
		Expect(err).NotTo(HaveOccurred())

		fi, err := tarball.Stat()
		Expect(err).NotTo(HaveOccurred())
		tarballName = fi.Name()
	})

	AfterEach(func() {
		err = os.Remove(tarballName)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Read", func() {
		It("extracts the release manifest information from the tarball", func() {
			var releaseManifest builder.Part
			releaseManifest, err = reader.Read(tarballName)
			Expect(err).NotTo(HaveOccurred())
			Expect(releaseManifest).To(Equal(builder.Part{
				File: tarballName,
				Name: "release",
				Metadata: builder.ReleaseManifest{
					Name:            "release",
					Version:         "1.2.3",
					File:            filepath.Base(tarballName),
					SHA1:            releaseSHA1,
					StemcellOS:      "ubuntu-xenial",
					StemcellVersion: "170.25",
				},
			}))
		})

		Context("when the release is not pre-compiled", func() {
			BeforeEach(func() {
				tarball, releaseSHA1, err = createReleaseTarball(`
name: release
version: 1.2.3
`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("extracts the release manifest information from the tarball", func() {
				var releaseManifest builder.Part
				releaseManifest, err = reader.Read(tarballName)
				Expect(err).NotTo(HaveOccurred())
				Expect(releaseManifest).To(Equal(builder.Part{
					File: tarballName,
					Name: "release",
					Metadata: builder.ReleaseManifest{
						Name:            "release",
						Version:         "1.2.3",
						File:            filepath.Base(tarballName),
						SHA1:            releaseSHA1,
						StemcellOS:      "",
						StemcellVersion: "",
					},
				}))
			})
		})

		Context("failure cases", func() {
			Context("when the tarball cannot be opened", func() {
				It("returns an error", func() {
					_, err = reader.Read("some-non-existing-file")
					Expect(err).To(MatchError(ContainSubstring("no such file")))
				})
			})

			Context("when the input is not a valid gzip", func() {
				It("returns an error", func() {
					tarballFile, err := os.OpenFile(tarballName, os.O_RDWR, 0666)
					Expect(err).NotTo(HaveOccurred())

					_, err = tarballFile.WriteAt([]byte{}, 10)
					Expect(err).NotTo(HaveOccurred())

					err = tarballFile.Close()
					Expect(err).NotTo(HaveOccurred())

					var contents []byte
					contents, err = ioutil.ReadFile(tarballName)
					Expect(err).NotTo(HaveOccurred())

					By("corrupting the gzip header contents", func() {
						contents[0] = 0
						err = ioutil.WriteFile(tarballName, contents, 0666)
						Expect(err).NotTo(HaveOccurred())
					})

					_, err = reader.Read(tarballName)
					Expect(err).To(MatchError("gzip: invalid header"))
				})
			})

			Context("when the header file is corrupt", func() {
				It("returns an error", func() {
					tarball, err = os.Create(tarballName)
					Expect(err).NotTo(HaveOccurred())

					gw := gzip.NewWriter(tarball)
					tw := tar.NewWriter(gw)

					Expect(tw.Close()).NotTo(HaveOccurred())
					Expect(gw.Close()).NotTo(HaveOccurred())

					_, err = reader.Read(tarballName)
					Expect(err).To(MatchError(fmt.Sprintf("could not find release.MF in %q", tarballName)))
				})
			})

			Context("when there is no release.MF", func() {
				It("returns an error", func() {
					tarball, err = os.Create(tarballName)
					Expect(err).NotTo(HaveOccurred())

					gw := gzip.NewWriter(tarball)
					tw := tar.NewWriter(gw)

					releaseManifest := bytes.NewBuffer([]byte(`---
name: release
version: 1.2.3
`))

					header := &tar.Header{
						Name:    "./someotherfile.MF",
						Size:    int64(releaseManifest.Len()),
						Mode:    int64(0644),
						ModTime: time.Now(),
					}

					err = tw.WriteHeader(header)
					Expect(err).NotTo(HaveOccurred())

					_, err = io.Copy(tw, releaseManifest)
					Expect(err).NotTo(HaveOccurred())

					err = tw.Close()
					Expect(err).NotTo(HaveOccurred())

					err = gw.Close()
					Expect(err).NotTo(HaveOccurred())

					_, err = reader.Read(tarballName)
					Expect(err).To(MatchError(fmt.Sprintf("could not find release.MF in %q", tarballName)))
				})
			})

			Context("when the tarball is corrupt", func() {
				It("returns an error", func() {
					tarball, err = os.Create(tarballName)
					Expect(err).NotTo(HaveOccurred())

					gw := gzip.NewWriter(tarball)
					tw := bufio.NewWriter(gw)

					_, err = tw.WriteString("I am a banana!")
					Expect(err).NotTo(HaveOccurred())

					err = tw.Flush()
					Expect(err).NotTo(HaveOccurred())

					err = gw.Close()
					Expect(err).NotTo(HaveOccurred())

					_, err = reader.Read(tarballName)
					Expect(err).To(MatchError(fmt.Sprintf("error while reading %q: unexpected EOF", tarballName)))
				})
			})

			Context("when the release manifest is not YAML", func() {
				It("returns an error", func() {
					tarball, err = os.Create(tarballName)
					Expect(err).NotTo(HaveOccurred())

					gw := gzip.NewWriter(tarball)
					tw := tar.NewWriter(gw)

					releaseManifest := bytes.NewBuffer([]byte(`%%%%%`))

					header := &tar.Header{
						Name:    "./release.MF",
						Size:    int64(releaseManifest.Len()),
						Mode:    int64(0644),
						ModTime: time.Now(),
					}

					err = tw.WriteHeader(header)
					Expect(err).NotTo(HaveOccurred())

					_, err = io.Copy(tw, releaseManifest)
					Expect(err).NotTo(HaveOccurred())

					err = tw.Close()
					Expect(err).NotTo(HaveOccurred())

					err = gw.Close()
					Expect(err).NotTo(HaveOccurred())

					_, err = reader.Read(tarballName)
					Expect(err).To(MatchError("yaml: could not find expected directive name"))
				})
			})
		})

		Context("when the release has a malformed stemcell string", func() {
			BeforeEach(func() {
				tarball, releaseSHA1 = createReleaseTarball(`
name: release
version: 1.2.3
compiled_packages:
- name: some-package
  stemcell: invalid
`)
			})

			It("extracts the release manifest information from the tarball", func() {
				_, err := reader.Read(tarballName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid"))
			})
		})
	})
})
