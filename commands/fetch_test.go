package commands_test

import (
	"errors"
	"fmt"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/release"

	"github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
)

var _ = Describe("Fetch", func() {
	var (
		fetch                      *Fetch
		logger                     *log.Logger
		tmpDir                     string
		releasesDirectory          string
		kilnfileLock               cargo.KilnfileLock
		s3CompiledReleaseSource    *fetcherFakes.ReleaseSource
		boshIOReleaseSource        *fetcherFakes.ReleaseSource
		s3BuiltReleaseSource       *fetcherFakes.ReleaseSource
		multiReleaseSourceProvider *fakes.MultiReleaseSourceProvider
		localReleaseDirectory      *fakes.LocalReleaseDirectory
	)

	const (
		s3CompiledReleaseSourceID = "s3-compiled"
		s3BuiltReleaseSourceID    = "s3-built"
		boshIOReleaseSourceID     = fetcher.ReleaseSourceTypeBOSHIO
	)

	Describe("Run", func() {
		BeforeEach(func() {
			logger = log.New(GinkgoWriter, "", 0)

			var err error
			tmpDir, err = ioutil.TempDir("", "fetch-test")

			releasesDirectory, err = ioutil.TempDir(tmpDir, "releases")
			Expect(err).NotTo(HaveOccurred())

			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name: "some-release", Version: "1.2.3",
						RemoteSource: s3CompiledReleaseSourceID, RemotePath: "my-remote-path",
					},
				},
				Stemcell: cargo.Stemcell{OS: "some-os", Version: "4.5.6"},
			}
			localReleaseDirectory = new(fakes.LocalReleaseDirectory)

			s3CompiledReleaseSource = new(fetcherFakes.ReleaseSource)
			s3CompiledReleaseSource.IDReturns(s3CompiledReleaseSourceID)
			boshIOReleaseSource = new(fetcherFakes.ReleaseSource)
			boshIOReleaseSource.IDReturns(boshIOReleaseSourceID)
			s3BuiltReleaseSource = new(fetcherFakes.ReleaseSource)
			s3BuiltReleaseSource.IDReturns(s3BuiltReleaseSourceID)
			multiReleaseSourceProvider = new(fakes.MultiReleaseSourceProvider)

			fetch = &Fetch{
				ReleasesDir:                releasesDirectory,
				Kilnfile:                   cargo.Kilnfile{},
				Logger:                     logger,
				LocalReleaseDirectory:      localReleaseDirectory,
				MultiReleaseSourceProvider: multiReleaseSourceProvider.Spy,
			}
		})

		JustBeforeEach(func() {
			multiReleaseSourceProvider.Returns(
				fetcher.NewMultiReleaseSource(
					s3CompiledReleaseSource,
					boshIOReleaseSource,
					s3BuiltReleaseSource,
				),
			)
			fetch.KilnfileLock = kilnfileLock
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		When("a local compiled release exists", func() {
			const (
				expectedStemcellOS      = "fooOS"
				expectedStemcellVersion = "0.2.0"
			)
			var (
				releaseID     release.ID
				releaseOnDisk release.Local
			)
			BeforeEach(func() {
				releaseID = release.ID{Name: "some-release", Version: "0.1.0"}
				localReleasePath := filepath.Join(releasesDirectory,
					fmt.Sprintf("%s-%s.tgz", releaseID.Name, releaseID.Version))
				s3CompiledReleaseSource.DownloadReleaseReturns(
					release.Local{
						ID:        releaseID,
						LocalPath: localReleasePath,
						SHA1:      "correct-sha",
					}, nil)
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name: releaseID.Name, Version: releaseID.Version,
							RemoteSource: s3CompiledReleaseSourceID, RemotePath: "not-used",
							SHA1: "correct-sha",
						},
					},
					Stemcell: cargo.Stemcell{OS: expectedStemcellOS, Version: expectedStemcellVersion},
				}
				fetch.NoConfirm = true

				Expect(
					ioutil.WriteFile(localReleasePath, []byte("some-release-contents"), 0600),
				).To(Succeed())
			})

			When("the release on disk has the wrong SHA1", func() {
				BeforeEach(func() {
					releaseOnDisk = release.Local{
						ID:        releaseID,
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
						SHA1:      "wrong-sha",
					}
					localReleaseDirectory.GetLocalReleasesReturns([]release.Local{releaseOnDisk}, nil)
				})

				It("deletes the file from disk", func() {
					Expect(fetch.Run(nil)).To(Succeed())

					Expect(s3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

					Expect(localReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := localReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveLen(1))
					Expect(extras).To(ConsistOf(releaseOnDisk))
				})
			})

			When("the release on disk has the correct SHA1", func() {
				BeforeEach(func() {
					releaseOnDisk = release.Local{
						ID:        releaseID,
						LocalPath: fmt.Sprintf("releases/%s-%s.tgz", releaseID.Name, releaseID.Version),
						SHA1:      "correct-sha",
					}
					localReleaseDirectory.GetLocalReleasesReturns([]release.Local{releaseOnDisk}, nil)
				})

				It("does not delete the file from disk", func() {
					Expect(fetch.Run(nil)).To(Succeed())

					Expect(s3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(0))

					Expect(localReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := localReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveLen(0))
				})
			})
		})

		Context("starting with no releases but all can be downloaded from their source (happy path)", func() {
			var (
				s3CompiledReleaseID = release.ID{Name: "lts-compiled-release", Version: "1.2.4"}
				s3BuiltReleaseID    = release.ID{Name: "lts-built-release", Version: "1.3.9"}
				boshIOReleaseID     = release.ID{Name: "boshio-release", Version: "1.4.16"}
			)
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name: "lts-compiled-release", Version: "1.2.4",
							RemoteSource: s3CompiledReleaseSourceID, RemotePath: "some-s3-key",
							SHA1: "correct-sha",
						},
						{
							Name: "lts-built-release", Version: "1.3.9",
							RemoteSource: s3BuiltReleaseSourceID, RemotePath: "some-other-s3-key",
							SHA1: "correct-sha",
						},
						{
							Name: "boshio-release", Version: "1.4.16",
							RemoteSource: boshIOReleaseSourceID, RemotePath: "some-bosh-io-url",
							SHA1: "correct-sha",
						},
					},
					Stemcell: cargo.Stemcell{OS: "some-os", Version: "30.1"},
				}

				s3CompiledReleaseSource.DownloadReleaseReturns(
					release.Local{ID: s3CompiledReleaseID, LocalPath: "local-path", SHA1: "correct-sha"},
					nil)

				s3BuiltReleaseSource.DownloadReleaseReturns(
					release.Local{ID: s3BuiltReleaseID, LocalPath: "local-path2", SHA1: "correct-sha"},
					nil)

				boshIOReleaseSource.DownloadReleaseReturns(
					release.Local{ID: boshIOReleaseID, LocalPath: "local-path3", SHA1: "correct-sha"},
					nil)

				localReleaseDirectory.GetLocalReleasesReturns(nil, nil)
			})

			It("fetches compiled release from s3 compiled release source", func() {
				Expect(fetch.Run(nil)).To(Succeed())
				Expect(s3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

				releasesDir, object, threads := s3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(releasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(object).To(Equal(
					release.Remote{ID: s3CompiledReleaseID, RemotePath: "some-s3-key", SourceID: s3CompiledReleaseSourceID},
				))
			})

			It("fetches built release from s3 built release source", func() {
				Expect(fetch.Run(nil)).To(Succeed())
				Expect(s3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				releasesDir, object, threads := s3BuiltReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(releasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(object).To(Equal(
					release.Remote{ID: s3BuiltReleaseID, RemotePath: "some-other-s3-key", SourceID: s3BuiltReleaseSourceID},
				))
			})

			It("fetches bosh.io release from bosh.io release source", func() {
				Expect(fetch.Run(nil)).To(Succeed())
				Expect(boshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				releasesDir, object, threads := boshIOReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(releasesDir).To(Equal(releasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(object).To(Equal(
					release.Remote{ID: boshIOReleaseID, RemotePath: "some-bosh-io-url", SourceID: boshIOReleaseSourceID},
				))
			})
		})

		Context("when all releases are already present in releases directory", func() {
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name: "some-release-from-local-dir", Version: "1.2.3",
							RemoteSource: s3CompiledReleaseSourceID, RemotePath: "not-used",
							SHA1: "correct-sha",
						},
					},
					Stemcell: cargo.Stemcell{OS: "some-os", Version: "4.5.6"},
				}

				someLocalReleaseID := release.ID{
					Name:    "some-release-from-local-dir",
					Version: "1.2.3",
				}
				localReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{ID: someLocalReleaseID, LocalPath: "/path/to/some/release", SHA1: "correct-sha"},
				}, nil)
			})

			It("no-ops", func() {
				Expect(fetch.Run(nil)).To(Succeed())

				Expect(s3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
				Expect(s3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
				Expect(boshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
			})
		})

		Context("when some releases are already present in output directory", func() {
			var (
				missingReleaseS3CompiledID   release.ID
				missingReleaseS3CompiledPath = "s3-key-some-missing-release-on-s3-compiled"
				missingReleaseBoshIOID       release.ID
				missingReleaseBoshIOPath     = "some-other-bosh-io-key"
				missingReleaseS3BuiltID      release.ID
				missingReleaseS3BuiltPath    = "s3-key-some-missing-release-on-s3-built"

				missingReleaseS3Compiled,
				missingReleaseBoshIO,
				missingReleaseS3Built release.Remote
			)
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name: "some-release", Version: "1.2.3",
							RemoteSource: s3BuiltReleaseSourceID, RemotePath: "not-used",
							SHA1: "correct-sha",
						},
						{
							Name: "some-tiny-release", Version: "1.2.3",
							RemoteSource: boshIOReleaseSourceID, RemotePath: "not-used2",
							SHA1: "correct-sha",
						},
						{
							Name: "some-missing-release-on-s3-compiled", Version: "4.5.6",
							RemoteSource: s3CompiledReleaseSourceID, RemotePath: missingReleaseS3CompiledPath,
							SHA1: "correct-sha",
						},
						{
							Name: "some-missing-release-on-boshio", Version: "5.6.7",
							RemoteSource: boshIOReleaseSourceID, RemotePath: missingReleaseBoshIOPath,
							SHA1: "correct-sha",
						},
						{
							Name: "some-missing-release-on-s3-built", Version: "8.9.0",
							RemoteSource: s3BuiltReleaseSourceID, RemotePath: missingReleaseS3BuiltPath,
							SHA1: "correct-sha",
						},
					},
					Stemcell: cargo.Stemcell{OS: "some-os", Version: "4.5.6"},
				}

				missingReleaseS3CompiledID = release.ID{Name: "some-missing-release-on-s3-compiled", Version: "4.5.6"}
				missingReleaseBoshIOID = release.ID{Name: "some-missing-release-on-boshio", Version: "5.6.7"}
				missingReleaseS3BuiltID = release.ID{Name: "some-missing-release-on-s3-built", Version: "8.9.0"}

				localReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{
						ID:        release.ID{Name: "some-release", Version: "1.2.3"},
						LocalPath: "path/to/some/release",
						SHA1:      "correct-sha",
					},
					{
						ID:        release.ID{Name: "some-tiny-release", Version: "1.2.3"},
						LocalPath: "path/to/some/tiny/release",
						SHA1:      "correct-sha",
					},
				}, nil)

				s3CompiledReleaseSource.DownloadReleaseReturns(release.Local{
					ID: missingReleaseS3CompiledID, LocalPath: "local-path-1", SHA1: "correct-sha",
				}, nil)

				boshIOReleaseSource.DownloadReleaseReturns(release.Local{
					ID: missingReleaseBoshIOID, LocalPath: "local-path-2", SHA1: "correct-sha",
				}, nil)

				s3BuiltReleaseSource.DownloadReleaseReturns(release.Local{
					ID: missingReleaseS3BuiltID, LocalPath: "local-path-3", SHA1: "correct-sha",
				}, nil)

				missingReleaseS3Compiled = release.Remote{ID: missingReleaseS3CompiledID, RemotePath: missingReleaseS3CompiledPath, SourceID: s3CompiledReleaseSourceID}
				missingReleaseBoshIO = release.Remote{ID: missingReleaseBoshIOID, RemotePath: missingReleaseBoshIOPath, SourceID: boshIOReleaseSourceID}
				missingReleaseS3Built = release.Remote{ID: missingReleaseS3BuiltID, RemotePath: missingReleaseS3BuiltPath, SourceID: s3BuiltReleaseSourceID}
			})

			It("downloads only the missing releases", func() {
				Expect(fetch.Run(nil)).To(Succeed())

				Expect(s3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object, _ := s3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseS3Compiled))

				Expect(boshIOReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object, _ = boshIOReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseBoshIO))

				Expect(s3BuiltReleaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, object, _ = s3BuiltReleaseSource.DownloadReleaseArgsForCall(0)
				Expect(object).To(Equal(missingReleaseS3Built))
			})

			Context("when download fails", func() {
				var (
					wrappedErr error
				)

				BeforeEach(func() {
					wrappedErr = errors.New("kaboom")
					s3CompiledReleaseSource.DownloadReleaseReturns(
						release.Local{},
						wrappedErr,
					)
				})

				It("returns an error", func() {
					err := fetch.Run(nil)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("download failed")))
					Expect(errors.Is(err, wrappedErr)).To(BeTrue())
				})
			})

			Context("when the downloaded release has the wrong sha1", func() {
				var badReleasePath string

				BeforeEach(func() {
					badReleasePath = filepath.Join(releasesDirectory, "local-path-3")

					s3BuiltReleaseSource.DownloadReleaseCalls(func(string, release.Remote, int) (release.Local, error) {
						f, err := os.Create(badReleasePath)
						Expect(err).NotTo(HaveOccurred())
						defer f.Close()

						return release.Local{
							ID: missingReleaseS3BuiltID, LocalPath: badReleasePath, SHA1: "wrong-sha",
						}, nil
					})
				})

				It("errors", func() {
					err := fetch.Run(nil)

					Expect(err).To(MatchError(ContainSubstring("incorrect SHA1")))
					Expect(err).To(MatchError(ContainSubstring(`"correct-sha"`)))
					Expect(err).To(MatchError(ContainSubstring(`"wrong-sha"`)))
				})

				It("deletes the release file from disk", func() {
					_, err := os.Stat(badReleasePath)
					Expect(err).To(HaveOccurred())
					Expect(os.IsNotExist(err)).To(BeTrue(), "Expected file %q not to exist, but got a different error: %v", badReleasePath, err)
				})
			})
		})

		Context("when there are extra releases locally that are not in the Kilnfile.lock", func() {
			var (
				boshIOReleaseID = release.ID{Name: "some-release", Version: "1.2.3"}
				localReleaseID  = release.ID{Name: "some-extra-release", Version: "1.2.3"}
			)
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name: "some-release", Version: "1.2.3",
							RemoteSource: s3CompiledReleaseSourceID, RemotePath: "not-used",
						},
					},
					Stemcell: cargo.Stemcell{OS: "some-os", Version: "4.5.6"},
				}

				localReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{ID: localReleaseID, LocalPath: "path/to/some/extra/release", SHA1: "correct-sha"},
				}, nil)

				boshIOReleaseSource.DownloadReleaseReturns(
					release.Local{ID: boshIOReleaseID, LocalPath: "local-path", SHA1: "correct-sha"},
					nil)

			})

			Context("in non-interactive mode", func() {
				BeforeEach(func() {
					fetch.NoConfirm = true
				})

				It("deletes the extra releases", func() {
					Expect(fetch.Run(nil)).To(Succeed())

					Expect(s3CompiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

					Expect(localReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))

					extras, noConfirm := localReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(extras).To(HaveLen(1))
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(ConsistOf(
						release.Local{
							ID:        release.ID{Name: "some-extra-release", Version: "1.2.3"},
							LocalPath: "path/to/some/extra/release",
							SHA1:      "correct-sha",
						},
					))
				})
			})

			Context("when # of download threads is specified", func() {
				BeforeEach(func() {
					fetch.DownloadThreads = 10
				})

				It("passes concurrency parameter to DownloadReleases", func() {
					Expect(fetch.Run(nil)).To(Succeed())
					_, _, threads := s3CompiledReleaseSource.DownloadReleaseArgsForCall(0)
					Expect(threads).To(Equal(10))
				})
			})

			Context("failure cases", func() {
				Context("when local releases cannot be accessed", func() {
					BeforeEach(func() {
						localReleaseDirectory.GetLocalReleasesReturns(nil, errors.New("some-error"))
					})
					It("returns an error", func() {
						err := fetch.Run(nil)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("some-error"))
					})
				})
			})
		})

	})
})
