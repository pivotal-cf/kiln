package helper_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/helper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem", func() {
	var filesystem helper.Filesystem

	BeforeEach(func() {
		filesystem = helper.NewFilesystem()
	})

	Describe("Open", func() {
		It("opens the specified file", func() {
			tempFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = tempFile.WriteString("file contents")
			Expect(err).NotTo(HaveOccurred())

			Expect(tempFile.Close()).To(Succeed())

			file, err := filesystem.Open(tempFile.Name())
			Expect(err).NotTo(HaveOccurred())

			contents, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal([]byte("file contents")))
		})

		Context("failure cases", func() {
			Context("when the file does not exist", func() {
				It("returns an error", func() {
					_, err := filesystem.Open("missing-file")
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})
		})
	})

	Describe("Walk", func() {
		It("traverses the specified path", func() {
			tempDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			f := filepath.Join(tempDir, "some-file")
			tempfile, err := os.Create(f)
			Expect(err).NotTo(HaveOccurred())

			err = tempfile.Close()
			Expect(err).NotTo(HaveOccurred())

			files := []string{}
			filesystem.Walk(tempDir, func(filePath string, info os.FileInfo, err error) error {
				files = append(files, filePath)
				return nil
			})

			Expect(files).To(Equal([]string{tempDir, f}))
		})
	})

})
