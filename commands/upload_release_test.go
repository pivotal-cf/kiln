package commands_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/fetcher"
	"io"
	"log"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

var _ = Describe("UploadRelease", func() {
	Context("Execute", func() {
		var (
			fs                    billy.Filesystem
			loader                *fakes.KilnfileLoader
			releaseSourcesFactory *fakes.ReleaseSourcesFactory
			nonReleaseUploader    *fetcherFakes.ReleaseSource
			releaseUploader       *fakes.ReleaseUploader

			uploadRelease commands.UploadRelease

			expectedReleaseSHA string
		)

		var writeReleaseTarball = func(path, name, version string) string {
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

		BeforeEach(func() {
			fs = memfs.New()
			loader = new(fakes.KilnfileLoader)

			nonReleaseUploader = new(fetcherFakes.ReleaseSource)
			nonReleaseUploader.IDReturns("lemon-bucket")
			releaseUploader = new(fakes.ReleaseUploader)
			releaseUploader.IDReturns("orange-bucket")
			releaseSourcesFactory = new(fakes.ReleaseSourcesFactory)
			releaseSourcesFactory.ReleaseSourcesReturns([]fetcher.ReleaseSource{nonReleaseUploader, releaseUploader})

			uploadRelease = commands.UploadRelease{
				FS:                    fs,
				KilnfileLoader:        loader,
				Logger:                log.New(GinkgoWriter, "", 0),
				ReleaseSourcesFactory: releaseSourcesFactory,
			}
			expectedReleaseSHA = writeReleaseTarball("banana-release.tgz", "banana", "1.2.3")
		})

		When("it receives a correct tarball path", func() {
			It("uploads the tarball to the release source", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(1))

				name, version, file := releaseUploader.UploadReleaseArgsForCall(0)
				Expect(name).To(Equal("banana"))
				Expect(version).To(Equal("1.2.3"))

				hash := sha1.New()
				_, err = io.Copy(hash, file)
				Expect(err).NotTo(HaveOccurred())

				releaseSHA := fmt.Sprintf("%x", hash.Sum(nil))
				Expect(releaseSHA).To(Equal(expectedReleaseSHA))
			})
		})

		When("the release tarball is invalid", func() {
			BeforeEach(func() {
				f, err := fs.Create("invalid-release.tgz")
				_, _ = f.Write([]byte("invalid"))
				f.Close()

				Expect(err).NotTo(HaveOccurred())
			})

			It("errors", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "invalid-release.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})
				Expect(err).To(MatchError(ContainSubstring("error reading the release manifest")))
			})
		})

		When("the given release source doesn't exist", func() {
			When("no release sources are s3 buckets", func() {
				BeforeEach(func() {
					releaseSourcesFactory.ReleaseSourcesReturns(nil)
				})

				It("returns an error without suggested release sources", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", "not-read-see-struct/Kilnfile",
						"--local-path", "banana-release.tgz",
						"--release-source", "orange-bucket",
						"--variables-file", "my-secrets",
					})

					Expect(err).To(MatchError(ContainSubstring("release source")))
				})
			})

			When("some release sources are s3 buckets", func() {
				It("returns an error that suggests valid release sources", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", "not-read-see-struct/Kilnfile",
						"--local-path", "banana-release.tgz",
						"--release-source", "no-such-release-source",
						"--variables-file", "my-secrets",
					})

					Expect(err).To(MatchError(ContainSubstring("orange-bucket")))
				})
			})
		})

		When("the upload fails", func() {
			BeforeEach(func() {
				releaseUploader.UploadReleaseReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("upload")))
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})
	})
})
