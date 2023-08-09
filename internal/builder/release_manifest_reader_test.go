package builder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/proofing"
	"path/filepath"

	"github.com/pivotal-cf/kiln/internal/builder"
)

var _ = Describe("ReleaseManifestReader", func() {
	var (
		reader builder.ReleaseManifestReader
		err    error

		nonPreCompiledTarballPath = filepath.Join("testdata", "bpm-1.1.21.tgz")
		compiledTarballPath       = filepath.Join("testdata", "bpm-1.1.21-ubuntu-xenial-621.463.tgz")
	)

	BeforeEach(func() {
		reader = builder.NewReleaseManifestReader()
	})

	Describe("Read", func() {
		It("extracts the release manifest information from the tarball", func() {
			var releaseManifest builder.Part
			releaseManifest, err = reader.Read(compiledTarballPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(releaseManifest).To(Equal(builder.Part{
				File: compiledTarballPath,
				Name: "bpm",
				Metadata: proofing.Release{
					Name:    "bpm",
					Version: "1.1.21",
					File:    filepath.Base(compiledTarballPath),

					SHA1:       "be5b1710f33128f6c864eae1d97effddb94dd3ac",
					CommitHash: "fd88358",
				},
			}))
		})

		Context("when the release is not pre-compiled", func() {
			It("extracts the release manifest information from the tarball", func() {
				var releaseManifest builder.Part
				releaseManifest, err = reader.Read(nonPreCompiledTarballPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(releaseManifest).To(Equal(builder.Part{
					File: nonPreCompiledTarballPath,
					Name: "bpm",
					Metadata: proofing.Release{
						Name:    "bpm",
						Version: "1.1.21",
						File:    filepath.Base(nonPreCompiledTarballPath),

						SHA1:       "519b78f2f3333a7b9c000bbef325e12a2f36996d",
						CommitHash: "fd88358",
					},
				}))
			})
		})

		Context("failure cases", func() {
			Context("when the tarball cannot be opened", func() {
				It("returns an error", func() {
					_, err = reader.Read("some-non-existing-file")
					Expect(err).To(MatchError(ContainSubstring("no such file")))
				})
			})
		})
	})
})
