package commands_test

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	testHelpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("PublishRelease", func() {
	Context("Execute", func() {
		var (
			fs                    billy.Filesystem
			releaseUploaderFinder *commandsFakes.ReleaseUploaderFinder
			releaseUploader       *fakes.ReleaseUploader

			uploadRelease commands.PublishRelease

			expectedReleaseSHA string
		)

		BeforeEach(func() {
			fs = memfs.New()

			releaseUploader = new(fakes.ReleaseUploader)
			releaseUploader.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
			releaseUploaderFinder = new(commandsFakes.ReleaseUploaderFinder)
			releaseUploaderFinder.Returns(releaseUploader, nil)

			uploadRelease = commands.PublishRelease{
				FS:                    fs,
				Logger:                log.New(GinkgoWriter, "", 0),
				ReleaseUploaderFinder: releaseUploaderFinder.Spy,
			}

			Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{})).NotTo(HaveOccurred())
			Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{})).NotTo(HaveOccurred())

			var err error
			expectedReleaseSHA, err = testHelpers.WriteReleaseTarball("banana-release.tgz", "banana", "1.2.3", fs)
			Expect(err).NotTo(HaveOccurred())
		})

		When("it receives a correct tarball path", func() {
			It("uploads the tarball to the release source", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--upload-target-id", "orange-bucket",
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
					releaseUploader.GetMatchedReleaseReturns(component.Lock{
						Name: "banana", Version: "1.2.3",
						RemotePath:   "banana/banana-1.2.3.tgz",
						RemoteSource: "orange-bucket",
					}, nil)
				})

				It("errors and does not upload", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--upload-target-id", "orange-bucket",
					})
					Expect(err).To(MatchError(ContainSubstring("already exists")))

					Expect(releaseUploader.GetMatchedReleaseCallCount()).To(Equal(1))

					requirement := releaseUploader.GetMatchedReleaseArgsForCall(0)
					Expect(requirement).To(Equal(component.Spec{Name: "banana", Version: "1.2.3"}))

					Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
				})
			})

			When("the release tarball is compiled", func() {
				BeforeEach(func() {
					_, err := testHelpers.WriteTarballWithFile("banana-release.tgz", "release.MF", `
name: banana
version: 1.2.3
compiled_packages:
- stemcell: plan9/42
`, fs)
					Expect(err).NotTo(HaveOccurred())
				})

				It("errors and does not upload", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-release.tgz",
						"--upload-target-id", "orange-bucket",
					})
					Expect(err).To(MatchError(ContainSubstring("compiled release")))
					Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
				})
			})

			When("the release version is not a finalized release", func() {
				var err error
				devReleases := []struct {
					tarballName string
					version     string
				}{
					{"banana-rc.tgz", "1.2.3-rc.100"},
					{"banana-build.tgz", "1.2.3-build.56"},
					{"banana-dev.tgz", "1.2.3-dev.14784"},
					{"banana-alpha.tgz", "1.2.3-alpha.1"},
				}

				BeforeEach(func() {
					for _, rel := range devReleases {
						_, err = testHelpers.WriteReleaseTarball(rel.tarballName, "banana", rel.version, fs)
						Expect(err).NotTo(HaveOccurred())
					}
				})

				It("errors with a descriptive message", func() {
					for _, rel := range devReleases {
						err := uploadRelease.Execute([]string{
							"--local-path", rel.tarballName,
							"--upload-target-id", "orange-bucket",
						})
						Expect(err).To(MatchError(ContainSubstring("only finalized releases are allowed")))
					}

					Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
				})
			})

			When("the release version is malformed", func() {
				BeforeEach(func() {
					_, err := testHelpers.WriteReleaseTarball("banana-malformed.tgz", "banana", "v1_2_garbage", fs)
					Expect(err).NotTo(HaveOccurred())
				})

				It("errors with a descriptive message", func() {
					err := uploadRelease.Execute([]string{
						"--local-path", "banana-malformed.tgz",
						"--upload-target-id", "orange-bucket",
					})
					Expect(err).To(MatchError(ContainSubstring("release version is not valid semver")))
					Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
				})
			})
		})

		When("the release tarball is invalid", func() {
			BeforeEach(func() {
				f, err := fs.Create("invalid-release.tgz")
				_, _ = f.Write([]byte("invalid"))
				defer closeAndIgnoreError(f)

				Expect(err).NotTo(HaveOccurred())
			})

			It("errors", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "invalid-release.tgz",
					"--upload-target-id", "orange-bucket",
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
						"--upload-target-id", "orange-bucket",
					})

					Expect(err).To(MatchError(ContainSubstring("no release source eligible")))
				})
			})
		})

		When("querying the release source fails", func() {
			BeforeEach(func() {
				releaseUploader.GetMatchedReleaseReturns(component.Lock{}, errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--upload-target-id", "orange-bucket",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})

			It("doesn't upload anything", func() {
				_ = uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--upload-target-id", "orange-bucket",
				})
				Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
			})
		})

		When("the upload fails", func() {
			BeforeEach(func() {
				releaseUploader.UploadReleaseReturns(component.Lock{}, errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--local-path", "banana-release.tgz",
					"--upload-target-id", "orange-bucket",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("upload")))
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})
	})
})
