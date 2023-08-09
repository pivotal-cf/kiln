package commands_test

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("UploadRelease", func() {
	Context("Execute", func() {
		var (
			releaseUploaderFinder *commandsFakes.ReleaseUploaderFinder
			releaseUploader       *fakes.ReleaseUploader

			uploadRelease commands.UploadRelease

			tileDirectory string
		)

		BeforeEach(func() {
			var err error
			tileDirectory, err = os.MkdirTemp("", "")
			if err != nil {
				log.Fatal(err)
			}

			Expect(writeYAML(filepath.Join(tileDirectory, "Kilnfile"), cargo.Kilnfile{})).NotTo(HaveOccurred())
			Expect(writeYAML(filepath.Join(tileDirectory, "Kilnfile.lock"), cargo.KilnfileLock{})).NotTo(HaveOccurred())

			releaseUploader = new(fakes.ReleaseUploader)
			releaseUploader.GetMatchedReleaseReturns(cargo.BOSHReleaseTarballLock{}, component.ErrNotFound)
			releaseUploaderFinder = new(commandsFakes.ReleaseUploaderFinder)
			releaseUploaderFinder.Returns(releaseUploader, nil)

			uploadRelease = commands.UploadRelease{
				Logger:                log.New(GinkgoWriter, "", 0),
				ReleaseUploaderFinder: releaseUploaderFinder.Spy,
			}
		})

		When("it receives a correct tarball path", func() {
			It("uploads the tarball to the release source", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
					"--local-path", filepath.Join("testdata", "bpm-1.1.21.tgz"),
					"--upload-target-id", "orange-bucket",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(1))

				spec, f := releaseUploader.UploadReleaseArgsForCall(0)
				Expect(spec.Name).To(Equal("bpm"))
				Expect(spec.Version).To(Equal("1.1.21"))

				file, ok := f.(*os.File)
				Expect(ok).To(BeTrue())

				Expect(file.Name()).To(Equal(filepath.Join("testdata", "bpm-1.1.21.tgz")))
			})

			When("the release already exists on the release source", func() {
				BeforeEach(func() {
					releaseUploader.GetMatchedReleaseReturns(cargo.BOSHReleaseTarballLock{
						Name: "banana", Version: "1.2.3",
						RemotePath:   "banana/banana-1.2.3.tgz",
						RemoteSource: "orange-bucket",
					}, nil)
				})

				It("errors and does not upload", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
						"--local-path", filepath.Join("testdata", "bpm-1.1.21.tgz"),
						"--upload-target-id", "orange-bucket",
					})
					Expect(err).To(MatchError(ContainSubstring("already exists")))

					Expect(releaseUploader.GetMatchedReleaseCallCount()).To(Equal(1))

					requirement := releaseUploader.GetMatchedReleaseArgsForCall(0)
					Expect(requirement).To(Equal(cargo.BOSHReleaseTarballSpecification{Name: "bpm", Version: "1.1.21"}))

					Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
				})
			})
		})

		When("the release tarball is invalid", func() {
			var invalidFilePath string
			BeforeEach(func() {
				invalidFilePath = filepath.Join(tileDirectory, "invalid-release.tgz")
				err := os.WriteFile(invalidFilePath, []byte("invalid"), 0o600)
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
					"--local-path", invalidFilePath,
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
						"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
						"--local-path", "banana-release.tgz",
						"--upload-target-id", "orange-bucket",
					})

					Expect(err).To(MatchError(ContainSubstring("no release source eligible")))
				})
			})
		})

		When("querying the release source fails", func() {
			BeforeEach(func() {
				releaseUploader.GetMatchedReleaseReturns(cargo.BOSHReleaseTarballLock{}, errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
					"--local-path", filepath.Join("testdata", "bpm-1.1.21.tgz"),
					"--upload-target-id", "orange-bucket",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})

			It("doesn't upload anything", func() {
				_ = uploadRelease.Execute([]string{
					"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
					"--local-path", "banana-release.tgz",
					"--upload-target-id", "orange-bucket",
				})
				Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
			})
		})

		When("the upload fails", func() {
			BeforeEach(func() {
				releaseUploader.UploadReleaseReturns(cargo.BOSHReleaseTarballLock{}, errors.New("boom"))
			})

			It("returns an error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", filepath.Join(tileDirectory, "Kilnfile"),
					"--local-path", filepath.Join("testdata", "bpm-1.1.21.tgz"),
					"--upload-target-id", "orange-bucket",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("upload")))
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})
	})
})
