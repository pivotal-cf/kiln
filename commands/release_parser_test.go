package commands_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReleaseParser", func() {
	Describe("Execute", func() {
		var (
			tempDir string
			reader  *fakes.PartReader
			parser  commands.ReleaseParser
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			file, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "some-release.tar.gz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "other-release.tgz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = ioutil.TempFile("", "not-release")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "not-release.banana"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			reader = &fakes.PartReader{}
			parser = commands.NewReleaseParser(reader)
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

			releases, err := parser.Execute([]string{tempDir})
			Expect(err).NotTo(HaveOccurred())
			Expect(releases).To(Equal(map[string]interface{}{
				"some-name":  "some-metadata",
				"other-name": "other-metadata",
			}))

			Expect(reader.ReadCallCount()).To(Equal(2))
			Expect(reader.ReadArgsForCall(0)).To(Equal(filepath.Join(tempDir, "other-release.tgz")))
			Expect(reader.ReadArgsForCall(1)).To(Equal(filepath.Join(tempDir, "some-release.tar.gz")))
		})

		Context("failure cases", func() {
			Context("when there is a directory that does not exist", func() {
				It("returns an error", func() {
					_, err := parser.Execute([]string{"missing-directory"})
					Expect(err).To(MatchError("lstat missing-directory: no such file or directory"))
				})
			})

			Context("when the release manifest reader fails", func() {
				It("returns an error", func() {
					reader.ReadReturns(builder.Part{}, errors.New("failed to read release manifest"))

					_, err := parser.Execute([]string{tempDir})
					Expect(err).To(MatchError("failed to read release manifest"))
				})
			})
		})
	})
})
