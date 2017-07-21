package builder_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/kiln/builder"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Zipper", func() {
	Describe("SetPath", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates and opens zip file for writing", func() {
			zipFile := filepath.Join(tmpDir, "file.zip")

			zipper := builder.NewZipper()

			err := zipper.SetPath(zipFile)
			Expect(err).ToNot(HaveOccurred())

			Expect(zipFile).To(BeARegularFile())

			err = zipper.Close()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("CreateFolder", func() {
		var (
			tmpDir     string
			pathToTile string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())

			pathToTile = filepath.Join(tmpDir, "tile.zip")
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the given path", func() {
			zipper := builder.NewZipper()

			err := zipper.SetPath(pathToTile)
			Expect(err).ToNot(HaveOccurred())

			err = zipper.CreateFolder("some/path/to/folder")
			Expect(err).NotTo(HaveOccurred())

			err = zipper.Close()
			Expect(err).NotTo(HaveOccurred())

			reader, err := zip.OpenReader(pathToTile)
			Expect(err).NotTo(HaveOccurred())

			Expect(reader.File).To(HaveLen(1))
			Expect(reader.File[0].Name).To(Equal("some/path/to/folder/"))
			Expect(reader.File[0].Mode().IsDir()).To(BeTrue())
		})

		It("does not append separator if already given to the input", func() {
			zipper := builder.NewZipper()

			err := zipper.SetPath(pathToTile)
			Expect(err).ToNot(HaveOccurred())

			err = zipper.CreateFolder("some/path/to/folder/")
			Expect(err).NotTo(HaveOccurred())

			err = zipper.Close()
			Expect(err).NotTo(HaveOccurred())

			reader, err := zip.OpenReader(pathToTile)
			Expect(err).NotTo(HaveOccurred())

			Expect(reader.File).To(HaveLen(1))
			Expect(reader.File[0].Name).To(Equal("some/path/to/folder/"))
			Expect(reader.File[0].Mode().IsDir()).To(BeTrue())
		})

		Context("failure cases", func() {
			Context("when path has not been set", func() {
				It("returns an error", func() {
					zipper := builder.NewZipper()

					err := zipper.CreateFolder("/blah/blah")
					Expect(err).To(MatchError("zipper path must be set"))
				})
			})
		})
	})

	Describe("Add", func() {
		var (
			tmpDir   string
			tileFile string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())

			tileFile = filepath.Join(tmpDir, "tile.zip")
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("writes the given file into the path", func() {
			zipper := builder.NewZipper()

			err := zipper.SetPath(tileFile)
			Expect(err).ToNot(HaveOccurred())

			err = zipper.Add("some/path/to/file.txt", strings.NewReader("file contents"))
			Expect(err).NotTo(HaveOccurred())

			err = zipper.Close()
			Expect(err).NotTo(HaveOccurred())

			reader, err := zip.OpenReader(tileFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(reader.File).To(HaveLen(1))
			Expect(reader.File[0].Name).To(Equal("some/path/to/file.txt"))

			file, err := reader.File[0].Open()
			Expect(err).NotTo(HaveOccurred())

			contents, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())

			Expect(contents).To(Equal([]byte("file contents")))
		})

		Context("failure cases", func() {
			Context("when the file cannot be copied", func() {
				It("returns an error", func() {
					buffer := NewBuffer(bytes.NewBuffer([]byte{}))
					buffer.Error = errors.New("failed to read file")

					zipper := builder.NewZipper()

					err := zipper.SetPath(tileFile)
					Expect(err).ToNot(HaveOccurred())

					err = zipper.Add("file.txt", buffer)
					Expect(err).To(MatchError("failed to read file"))
				})
			})

			Context("when path has not been set", func() {
				It("returns an error", func() {
					zipper := builder.NewZipper()

					err := zipper.Add("/blah/blah", strings.NewReader("file contents"))
					Expect(err).To(MatchError("zipper path must be set"))
				})
			})
		})
	})
})
