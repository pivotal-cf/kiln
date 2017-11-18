package builder_test

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"
)

var _ = Describe("IconEncoder", func() {
	var (
		filesystem *fakes.Filesystem

		expectedBase64EncodedString string

		encoder builder.IconEncoder
	)

	BeforeEach(func() {
		filesystem = &fakes.Filesystem{}

		sampleData := []byte("this-is-some-data-to-encode")
		expectedBase64EncodedString = base64.StdEncoding.EncodeToString(sampleData)

		testFile := ioutil.NopCloser(strings.NewReader(string(sampleData)))

		filesystem.OpenReturns(testFile, nil)

		encoder = builder.NewIconEncoder(filesystem)
	})

	Describe("Encode", func() {
		It("opens the file for reading", func() {
			base64EncodedString, err := encoder.Encode("some-path")
			Expect(err).NotTo(HaveOccurred())

			Expect(filesystem.OpenCallCount()).To(Equal(1))
			Expect(filesystem.OpenArgsForCall(0)).To(Equal("some-path"))

			Expect(base64EncodedString).To(Equal(expectedBase64EncodedString))
		})

		Context("failure cases", func() {
			Context("when opening the file fails", func() {
				BeforeEach(func() {
					filesystem.OpenReturns(nil, errors.New("kaboom!"))
				})

				It("returns the error", func() {
					_, err := encoder.Encode("some-path")
					Expect(err).To(MatchError("kaboom!"))
				})
			})

			Context("when reading the icon file fails", func() {
				var tempfile *os.File

				BeforeEach(func() {
					var err error
					tempfile, err = ioutil.TempFile("", "broken-file")
					Expect(err).NotTo(HaveOccurred())
					Expect(tempfile.Close()).To(Succeed())

					filesystem.OpenReturns(tempfile, nil)
				})

				AfterEach(func() {
					Expect(os.RemoveAll(tempfile.Name())).To(Succeed())
				})

				It("returns the error", func() {
					_, err := encoder.Encode("some-path")
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
