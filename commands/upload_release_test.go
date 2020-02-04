package commands_test

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

var _ = Describe("UploadRelease", func() {
	Context("Execute", func() {
		var (
			fs                    billy.Filesystem
			loader                *fakes.KilnfileLoader
			releaseUploaderFinder *fakes.ReleaseUploaderFinder
			releaseUploader       *fetcherFakes.ReleaseUploader

			uploadRelease commands.UploadRelease

			expectedReleaseSHA string
		)

		BeforeEach(func() {
			fs = memfs.New()
			loader = new(fakes.KilnfileLoader)

			releaseUploader = new(fetcherFakes.ReleaseUploader)
			releaseUploaderFinder = new(fakes.ReleaseUploaderFinder)
			releaseUploaderFinder.Returns(releaseUploader, nil)

			uploadRelease = commands.UploadRelease{
				FS:                    fs,
				KilnfileLoader:        loader,
				Logger:                log.New(GinkgoWriter, "", 0),
				ReleaseUploaderFinder: releaseUploaderFinder.Spy,
			}

			var err error
			expectedReleaseSHA, err = test_helpers.WriteReleaseTarball("banana-release.tgz", "banana", "1.2.3", fs)
			Expect(err).NotTo(HaveOccurred())
		})

		When("it receives a correct tarball path", func() {
			It("uploads the tarball to the release source", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(1))

				spec, file := releaseUploader.UploadReleaseArgsForCall(0)
				Expect(spec.Name).To(Equal("banana"))
				Expect(spec.Version).To(Equal("1.2.3"))

				hash := sha1.New()
				_, err = io.Copy(hash, file)
				Expect(err).NotTo(HaveOccurred())

				releaseSHA := fmt.Sprintf("%x", hash.Sum(nil))
				Expect(releaseSHA).To(Equal(expectedReleaseSHA))
			})

			When("the release already exists on the release source", func() {
				BeforeEach(func() {
					releaseUploader.GetMatchedReleaseReturns(release.Remote{
						ID:         release.ID{Name: "banana", Version: "1.2.3"},
						RemotePath: "banana/banana-1.2.3.tgz",
						SourceID:   "orange-bucket",
					}, true, nil)
				})

				It("errors and does not upload", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--release-source", "orange-bucket",
					})
					Expect(err).To(MatchError(ContainSubstring("already exists")))

					Expect(releaseUploader.GetMatchedReleaseCallCount()).To(Equal(1))

					requirement := releaseUploader.GetMatchedReleaseArgsForCall(0)
					Expect(requirement).To(Equal(release.Requirement{Name: "banana", Version: "1.2.3"}))

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
			When("there's an error finding the release source", func() {
				BeforeEach(func() {
					releaseUploaderFinder.Returns(nil, errors.New("no release source eligible"))
				})

				It("returns the error", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--release-source", "orange-bucket",
					})

					Expect(err).To(MatchError(ContainSubstring("no release source eligible")))
				})
			})
		})

		When("querying the release source fails", func() {
			BeforeEach(func() {
				releaseUploader.GetMatchedReleaseReturns(release.Remote{}, false, errors.New("boom"))
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
