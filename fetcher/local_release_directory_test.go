package fetcher_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/release"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/builder"
	. "github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("LocalReleaseDirectory", func() {
	var (
		localReleaseDirectory LocalReleaseDirectory
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

		localReleaseDirectory = NewLocalReleaseDirectory(fakeLogger, releasesService)
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
				Expect(releases).To(ConsistOf(
					release.NewCompiledRelease(
						release.ReleaseID{Name: "some-release", Version: "1.2.3"},
						"some-os",
						"4.5.6",
						releaseFile,
					),
				))
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
		var extraFilePath string
		BeforeEach(func() {
			extraFilePath = filepath.Join(releasesDir, "extra-release-0.0-os-0-0.0.0.tgz")
			err := ioutil.WriteFile(extraFilePath, []byte("abc"), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes specified files", func() {
			extraReleaseID := release.ReleaseID{Name: "extra-release", Version: "0.0"}
			extraRelease := release.LocalRelease{ReleaseID: extraReleaseID, LocalPath: extraFilePath}

			extraReleases := release.LocalReleaseSet{
				extraReleaseID: extraRelease,
			}

			err := localReleaseDirectory.DeleteExtraReleases(extraReleases, noConfirm)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(extraFilePath)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		Context("when a file cannot be removed", func() {
			It("returns an error", func() {
				extraReleaseID := release.ReleaseID{Name: "extra-release-that-cannot-be-deleted", Version: "0.0"}
				extraRelease := release.LocalRelease{ReleaseID: extraReleaseID, LocalPath: "file-does-not-exist"}

				extraReleases := release.LocalReleaseSet{}
				extraReleases[extraReleaseID] = extraRelease

				err := localReleaseDirectory.DeleteExtraReleases(extraReleases, noConfirm)
				Expect(err).To(MatchError("failed to delete release extra-release-that-cannot-be-deleted"))
			})
		})
	})

	Describe("VerifyChecksums", func() {
		var (
			downloadedReleases release.LocalReleaseSet
			kilnfileLock       cargo.KilnfileLock
			goodFilePath       string
			badFilePath        string
			err                error
		)

		BeforeEach(func() {
			goodFilePath = filepath.Join(releasesDir, "good-1.2.3-ubuntu-xenial-190.0.0.tgz")
			err = ioutil.WriteFile(goodFilePath, []byte("abc"), 0644)
			Expect(err).NotTo(HaveOccurred())

			badFilePath = filepath.Join(releasesDir, "bad-1.2.3-ubuntu-xenial-190.0.0.tgz")
			err = ioutil.WriteFile(badFilePath, []byte("some bad sha file"), 0644)
			Expect(err).NotTo(HaveOccurred())

			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:    "good",
						Version: "1.2.3",
						SHA1:    "a9993e364706816aba3e25717850c26c9cd0d89d", // sha1 for string "abc"
					},
					{
						Name:    "bad",
						Version: "1.2.3",
						SHA1:    "a9993e364706816aba3e25717850c26c9cd0d89d", // sha1 for string "abc"
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "ubuntu-xenial",
					Version: "190.0.0",
				},
			}
		})

		Context("when all the checksums on the downloaded releases match their checksums in Kilnfile.lock", func() {
			It("succeeds", func() {
				downloadedReleases = release.LocalReleaseSet{
					release.ReleaseID{Name: "good", Version: "1.2.3"}: release.LocalRelease{
						ReleaseID: release.ReleaseID{Name: "good", Version: "1.2.3"},
						LocalPath: goodFilePath,
					},
				}
				err := localReleaseDirectory.VerifyChecksums(downloadedReleases, kilnfileLock)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when at least one checksum on the downloaded releases does not match the checksum in Kilnfile.lock", func() {
			It("returns an error and deletes the bad release", func() {
				downloadedReleases = release.LocalReleaseSet{
					release.ReleaseID{Name: "bad", Version: "1.2.3"}: release.LocalRelease{
						ReleaseID: release.ReleaseID{Name: "bad", Version: "1.2.3"},
						LocalPath: badFilePath,
					},
				}
				err := localReleaseDirectory.VerifyChecksums(downloadedReleases, kilnfileLock)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("These downloaded releases do not match the checksum"))

				_, err = os.Stat(badFilePath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		Context("when no checksum is specified for a release (and the release file is not in the normal place)", func() {
			var (
				nonStandardFilePath string
			)

			BeforeEach(func() {
				nonStandardFilePath = filepath.Join(releasesDir, "uaa-release-73.0.0.tgz") // bosh.io name, different from s3
				err = ioutil.WriteFile(nonStandardFilePath, []byte("some release file"), 0644)
				Expect(err).NotTo(HaveOccurred())
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name:    "good",
							Version: "1.2.3",
							SHA1:    "a9993e364706816aba3e25717850c26c9cd0d89d",
						},
						{Name: "uaa", Version: "7.3.0"},
					},
					Stemcell: cargo.Stemcell{
						OS:      "ubuntu-xenial",
						Version: "190.0.0",
					},
				}
			})

			It("does not validate its checksum", func() {
				downloadedReleases = release.LocalReleaseSet{
					release.ReleaseID{Name: "good", Version: "1.2.3"}: release.LocalRelease{
						ReleaseID: release.ReleaseID{Name: "good", Version: "1.2.3"},
						LocalPath: goodFilePath,
					},
					release.ReleaseID{Name: "uaa", Version: "7.3.0"}: release.LocalRelease{
						ReleaseID: release.ReleaseID{Name: "uaa", Version: "7.3.0"},
						LocalPath: nonStandardFilePath,
					},
				}
				err := localReleaseDirectory.VerifyChecksums(downloadedReleases, kilnfileLock)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
