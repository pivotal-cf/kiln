package baking_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/baking/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Checksummer", func() {
	var (
		logger      *fakes.Logger
		checksummer Checksummer
		tmpdir      string
	)

	BeforeEach(func() {
		logger = &fakes.Logger{}
		checksummer = NewChecksummer(logger)

		var err error
		tmpdir, err = os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())

		file, err := os.Create(filepath.Join(tmpdir, "fixture"))
		Expect(err).NotTo(HaveOccurred())

		fixture, err := os.Open("fixtures/file.txt")
		Expect(err).NotTo(HaveOccurred())

		_, err = io.Copy(file, fixture)
		Expect(err).NotTo(HaveOccurred())

		Expect(file.Close()).To(Succeed())
		Expect(fixture.Close()).To(Succeed())
	})

	AfterEach(func() {
		err := os.Chmod(tmpdir, 0o777)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.RemoveAll(tmpdir)).To(Succeed())
	})

	It("prints the sha256 checksum of the file at the given path", func() {
		path := filepath.Join(tmpdir, "fixture")
		err := checksummer.Sum(path)
		Expect(err).NotTo(HaveOccurred())

		Expect(logger.PrintlnCallCount()).To(Equal(2))

		line := logger.PrintlnArgsForCall(0)[0]
		Expect(line).To(Equal(fmt.Sprintf("Calculating SHA256 checksum of %s...", path)))

		line = logger.PrintlnArgsForCall(1)[0]
		Expect(line).To(Equal("SHA256 checksum: 2a89f69f18679fef3a1f833d1c5e561cc24ea02ce85b3fb7fae21dd971c9c9cd"))
	})

	It("writes the checksum to a .sha256 file in the same directory as the output file", func() {
		path := filepath.Join(tmpdir, "fixture")
		err := checksummer.Sum(path)
		Expect(err).NotTo(HaveOccurred())

		contents, err := os.ReadFile(fmt.Sprintf("%s.sha256", path))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("2a89f69f18679fef3a1f833d1c5e561cc24ea02ce85b3fb7fae21dd971c9c9cd"))
	})

	Context("when the directory does not have write permissions", func() {
		It("returns an error", func() {
			err := os.Chmod(tmpdir, 0o544)
			Expect(err).NotTo(HaveOccurred())

			path := filepath.Join(tmpdir, "fixture")
			err = checksummer.Sum(path)
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("open %s.sha256: permission denied", path))))
		})
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			err := checksummer.Sum("fixtures/does-not-exist.txt")
			Expect(err).To(MatchError(ContainSubstring("open fixtures/does-not-exist.txt: no such file or directory")))
		})
	})
})
