package commands_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/release"
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
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
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

			When("the release already exists on the release source", func() {
				BeforeEach(func() {
					releaseUploader.GetMatchedReleasesReturns([]release.RemoteRelease{
						{
							ReleaseID:  release.ReleaseID{Name: "banana", Version: "1.2.3"},
							RemotePath: "banana/banana-1.2.3.tgz",
						},
					}, nil)
				})

				It("errors and does not upload", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--release-source", "orange-bucket",
					})
					Expect(err).To(MatchError(ContainSubstring("already exists")))

					Expect(releaseUploader.GetMatchedReleasesCallCount()).To(Equal(1))

					requirementSet := releaseUploader.GetMatchedReleasesArgsForCall(0)
					Expect(requirementSet).To(HaveLen(1))
					Expect(requirementSet).To(HaveKeyWithValue(
						release.ReleaseID{Name: "banana", Version: "1.2.3"},
						release.ReleaseRequirement{Name: "banana", Version: "1.2.3"},
					))

					Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
				})
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
					"--local-path", "invalid-release.tgz",
					"--release-source", "orange-bucket",
				})
				Expect(err).To(MatchError(ContainSubstring("error reading the release manifest")))
			})
		})

		When("the given release source doesn't exist", func() {
			When("no release sources can upload", func() {
				BeforeEach(func() {
					releaseSourcesFactory.ReleaseSourcesReturns(nil)
				})

				It("returns an error without suggested release sources", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--release-source", "orange-bucket",
					})

					Expect(err).To(MatchError(ContainSubstring("release source")))
				})
			})

			When("some release sources can upload", func() {
				It("returns an error that suggests valid release sources", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--release-source", "no-such-release-source",
					})

					Expect(err).To(MatchError(ContainSubstring("orange-bucket")))
				})
			})
		})

		When("querying the release source fails", func() {
			BeforeEach(func() {
				releaseUploader.GetMatchedReleasesReturns(nil, errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})

			It("doesn't upload anything", func() {
				_ = uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
				})
				Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
			})
		})

		When("the upload fails", func() {
			BeforeEach(func() {
				releaseUploader.UploadReleaseReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("upload")))
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})
	})
})
