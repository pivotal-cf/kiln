package baking_test

import (
	"errors"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/baking/fakes"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StemcellService", func() {
	Describe("FromDirectories", func() {
		var (
			tempDir string
			logger  *fakes.Logger
			reader  *fakes.PartReader
			service baking.StemcellService
		)

		BeforeEach(func() {
			logger = &fakes.Logger{}
			reader = &fakes.PartReader{}
			service = baking.NewStemcellService(logger, reader)

			var err error
			tempDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			file, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "some-stemcell.tar.gz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "other-stemcell.tgz"))).To(Succeed())
			Expect(file.Close()).To(Succeed())

			file, err = ioutil.TempFile("", "not-stemcell")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Rename(file.Name(), filepath.Join(tempDir, "not-stemcell.banana"))).To(Succeed())
			Expect(file.Close()).To(Succeed())
		})

		It("walks directory for all stemcells to parse", func() {
			reader.ReadReturnsOnCall(0, builder.Part{
				Metadata: builder.StemcellManifest{
					Version:         "some-version",
					OperatingSystem: "some-os",
				},
			}, nil)

			reader.ReadReturnsOnCall(1, builder.Part{
				Metadata: builder.StemcellManifest{
					Version:         "some-other-version",
					OperatingSystem: "some-other-os",
				},
			}, nil)

			stemcell, err := service.FromDirectories([]string{tempDir})
			Expect(err).NotTo(HaveOccurred())
			Expect(stemcell).To(Equal(map[string]interface{}{
				"some-os": builder.StemcellManifest{
					Version:         "some-version",
					OperatingSystem: "some-os",
				},
				"some-other-os": builder.StemcellManifest{
					Version:         "some-other-version",
					OperatingSystem: "some-other-os",
				},
			}))
		})

		It("warns if multiple OS versions are found", func() {
			reader.ReadReturnsOnCall(0, builder.Part{
				Metadata: builder.StemcellManifest{
					Version:         "version1",
					OperatingSystem: "some-os",
				},
			}, nil)

			reader.ReadReturnsOnCall(1, builder.Part{
				Metadata: builder.StemcellManifest{
					Version:         "version2",
					OperatingSystem: "some-os",
				},
			}, nil)

			_, err := service.FromDirectories([]string{tempDir})
			Expect(err).To(MatchError("more than one OS version was found for OS 'some-os' when parsing stemcells"))
		})
	})

	Describe("FromTarball", func() {
		var (
			logger  *fakes.Logger
			reader  *fakes.PartReader
			service baking.StemcellService
		)

		BeforeEach(func() {
			logger = &fakes.Logger{}
			reader = &fakes.PartReader{}
			reader.ReadReturns(builder.Part{
				Metadata: builder.StemcellManifest{
					Version:         "some-version",
					OperatingSystem: "some-os",
				},
			}, nil)

			service = baking.NewStemcellService(logger, reader)
		})

		It("parses the stemcell passed as a tarball", func() {
			stemcell, err := service.FromTarball("some-stemcell-tarball")
			Expect(err).NotTo(HaveOccurred())
			Expect(stemcell).To(Equal(builder.StemcellManifest{
				Version:         "some-version",
				OperatingSystem: "some-os",
			}))

			Expect(logger.PrintlnCallCount()).To(Equal(1))
			Expect(logger.PrintlnArgsForCall(0)).To(Equal([]interface{}{"Reading stemcell manifest..."}))

			Expect(reader.ReadCallCount()).To(Equal(1))
			Expect(reader.ReadArgsForCall(0)).To(Equal("some-stemcell-tarball"))
		})

		Context("when there is no tarball to parse", func() {
			It("returns nothing", func() {
				stemcell, err := service.FromTarball("")
				Expect(err).NotTo(HaveOccurred())
				Expect(stemcell).To(BeNil())

				Expect(logger.PrintlnCallCount()).To(Equal(0))
				Expect(reader.ReadCallCount()).To(Equal(0))
			})
		})

		Context("failure cases", func() {
			Context("when the reader fails", func() {
				It("returns an error", func() {
					reader.ReadReturns(builder.Part{}, errors.New("failed to read"))

					_, err := service.FromTarball("some-stemcell-tarball")
					Expect(err).To(MatchError("failed to read"))
				})
			})
		})
	})
})
