package baking_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/baking/fakes"
	"github.com/pivotal-cf/kiln/internal/builder"
)

var _ = Describe("ReleasesService", func() {
	Describe("FromDirectories", func() {
		var (
			tempDir string
			logger  *fakes.Logger
			reader  *fakes.PartReader
			service ReleasesService
		)

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			file, err := os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "some-release.tar.gz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "other-release.tgz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = os.CreateTemp("", "not-release")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "not-release.banana"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			logger = new(fakes.Logger)
			reader = new(fakes.PartReader)
			service = NewReleasesService(logger, reader)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		It("parses the releases passed in a set of directories", func() {
			reader.ReadReturnsOnCall(0, builder.Part{
				File:     "some-file",
				Name:     "some-name",
				Metadata: "some-metadata",
			}, nil)

			reader.ReadReturnsOnCall(1, builder.Part{
				File:     "other-file",
				Name:     "other-name",
				Metadata: "other-metadata",
			}, nil)

			releases, err := service.FromDirectories([]string{tempDir})
			Expect(err).NotTo(HaveOccurred())
			Expect(releases).To(Equal(map[string]any{
				"some-name":  "some-metadata",
				"other-name": "other-metadata",
			}))

			Expect(logger.PrintlnCallCount()).To(Equal(1))
			Expect(logger.PrintlnArgsForCall(0)).To(Equal([]any{"Reading release manifests..."}))

			Expect(reader.ReadCallCount()).To(Equal(2))
			Expect(reader.ReadArgsForCall(0)).To(Equal(filepath.Join(tempDir, "other-release.tgz")))
			Expect(reader.ReadArgsForCall(1)).To(Equal(filepath.Join(tempDir, "some-release.tar.gz")))
		})

		Context("failure cases", func() {
			Context("when there is a directory that does not exist", func() {
				It("returns an error", func() {
					_, err := service.FromDirectories([]string{"missing-directory"})
					Expect(err).To(MatchError("lstat missing-directory: no such file or directory"))
				})
			})

			Context("when the release manifest reader fails", func() {
				It("returns an error", func() {
					reader.ReadReturns(builder.Part{}, errors.New("failed to read release manifest"))

					_, err := service.FromDirectories([]string{tempDir})
					Expect(err).To(MatchError("failed to read release manifest"))
				})
			})
		})
	})

	Describe("ReleasesInDirectory", func() {
		var (
			tempDir   string
			nestedDir string
			logger    *fakes.Logger
			reader    *fakes.PartReader
			service   ReleasesService
		)

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			nestedDir = filepath.Join(tempDir, "nested")
			Expect(os.Mkdir(nestedDir, 0o700)).To(Succeed())

			file, err := os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(nestedDir, "some-release.tar.gz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(nestedDir, "other-release.tgz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = os.CreateTemp("", "not-release")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(nestedDir, "not-release.banana"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			logger = new(fakes.Logger)
			reader = new(fakes.PartReader)
			service = NewReleasesService(logger, reader)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		It("parses the releases passed in a set of directories", func() {
			release1 := builder.Part{
				File:     "some-file",
				Name:     "some-name",
				Metadata: "some-metadata",
			}
			reader.ReadReturnsOnCall(0, release1, nil)

			release2 := builder.Part{
				File:     "other-file",
				Name:     "other-name",
				Metadata: "other-metadata",
			}
			reader.ReadReturnsOnCall(1, release2, nil)

			releases, err := service.ReleasesInDirectory(tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(releases).To(Equal([]builder.Part{release1, release2}))

			Expect(reader.ReadCallCount()).To(Equal(2))
			Expect(reader.ReadArgsForCall(0)).To(Equal(filepath.Join(nestedDir, "other-release.tgz")))
			Expect(reader.ReadArgsForCall(1)).To(Equal(filepath.Join(nestedDir, "some-release.tar.gz")))
		})

		Context("failure cases", func() {
			Context("when there is a directory that does not exist", func() {
				It("returns an error", func() {
					_, err := service.ReleasesInDirectory("missing-directory")
					Expect(err).To(MatchError("lstat missing-directory: no such file or directory"))
				})
			})

			Context("when the release manifest reader fails", func() {
				It("returns an error", func() {
					reader.ReadReturns(builder.Part{}, errors.New("failed to read release manifest"))

					_, err := service.ReleasesInDirectory(tempDir)
					Expect(err).To(MatchError("failed to read release manifest"))
				})
			})
		})
	})
})
