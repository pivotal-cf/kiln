package fetcher_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("LocalReleaseDirectory", func() {
	var (
		localReleaseDirectory fetcher.LocalReleaseDirectory
		noConfirm             bool
		releasesDir           string
		releaseFile           string
		fakeLogger            *log.Logger
	)

	BeforeEach(func() {
		var err error
		releasesDir, err = ioutil.TempDir("", "releases")
		noConfirm = true
		Expect(err).NotTo(HaveOccurred())

		releaseFile = filepath.Join(releasesDir, "some-release.tgz")

		fakeLogger = log.New(GinkgoWriter, "", 0)
		releaseManifestReader := builder.NewReleaseManifestReader()
		releasesService := baking.NewReleasesService(fakeLogger, releaseManifestReader)

		localReleaseDirectory = fetcher.NewLocalReleaseDirectory(releasesService)
	})

	AfterEach(func() {
		_ = os.RemoveAll(releasesDir)
	})

	Describe("GetLocalReleases", func() {
		Context("when releases exist in the releases dir", func() {
			BeforeEach(func() {
				fixtureContent, err := ioutil.ReadFile(filepath.Join("fixtures", "some-release.tgz"))
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(releaseFile, fixtureContent, 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a map of releases to locations", func() {
				releases, err := localReleaseDirectory.GetLocalReleases(releasesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(releases).To(HaveLen(1))
				Expect(releases).To(HaveKeyWithValue(cargo.CompiledRelease{
					Name:            "some-release",
					Version:         "1.2.3",
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}, releaseFile))
			})
		})

		Context("when there are no local releases", func() {
			It("returns an empty slice", func() {
				releases, err := localReleaseDirectory.GetLocalReleases(releasesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(releases).To(HaveLen(0))
			})
		})

		Context("when the releases directory does not exist", func() {
			It("returns an empty slice", func() {
				_, err := localReleaseDirectory.GetLocalReleases("some-invalid-directory")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("some-invalid-directory"))
			})
		})
	})

	Describe("DeleteExtraReleases", func() {
		var extraFile *os.File
		BeforeEach(func() {
			var err error
			extraFile, err = ioutil.TempFile(releasesDir, "extra-release")
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes specified files", func() {
			extraRelease := cargo.CompiledRelease{
				Name:            "extra-release",
				Version:         "v0.0",
				StemcellOS:      "os-0",
				StemcellVersion: "v0.0.0",
			}

			extraFileName := extraFile.Name()
			extraReleases := map[cargo.CompiledRelease]string{}
			extraReleases[extraRelease] = extraFileName

			err := localReleaseDirectory.DeleteExtraReleases(releasesDir, extraReleases, noConfirm)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(extraFile.Name())
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		Context("when a file cannot be removed", func() {
			It("returns an error", func() {
				extraRelease := cargo.CompiledRelease{
					Name:            "extra-release-that-cannot-be-deleted",
					Version:         "v0.0",
					StemcellOS:      "os-0",
					StemcellVersion: "v0.0.0",
				}

				extraReleases := map[cargo.CompiledRelease]string{}
				extraReleases[extraRelease] = "file-does-not-exist"

				err := localReleaseDirectory.DeleteExtraReleases(releasesDir, extraReleases, noConfirm)
				Expect(err).To(MatchError("failed to delete extra release extra-release-that-cannot-be-deleted"))
			})
		})
	})
})
