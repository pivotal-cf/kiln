package commands_test

import (
	"errors"
	"fmt"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"io/ioutil"
	"log"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
)

var _ = Describe("Fetch", func() {
	var (
		fetch                       Fetch
		logger                      *log.Logger
		tmpDir                      string
		kilnfile                    cargo.Kilnfile
		kilnfileLock                cargo.KilnfileLock
		someReleasesDirectory       string
		fakeS3CompiledReleaseSource *fetcherFakes.ReleaseSource
		fakeBoshIOReleaseSource     *fetcherFakes.ReleaseSource
		fakeS3BuiltReleaseSource    *fetcherFakes.ReleaseSource
		fakeReleaseSources          []fetcher.ReleaseSource
		fakeLocalReleaseDirectory   *fakes.LocalReleaseDirectory
		releaseSourcesFactory       *fakes.ReleaseSourcesFactory

		fetchExecuteArgs []string
		fetchExecuteErr  error
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(GinkgoWriter, "", 0)

			var err error
			tmpDir, err = ioutil.TempDir("", "fetch-test")

			someReleasesDirectory, err = ioutil.TempDir(tmpDir, "")
			Expect(err).NotTo(HaveOccurred())

			kilnfile = cargo.Kilnfile{}
			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.Release{{Name: "some-release", Version: "1.2.3"}},
				Stemcell: cargo.Stemcell{OS: "some-os", Version: "4.5.6"},
			}

			fakeLocalReleaseDirectory = new(fakes.LocalReleaseDirectory)

			fakeS3CompiledReleaseSource = new(fetcherFakes.ReleaseSource)
			fakeBoshIOReleaseSource = new(fetcherFakes.ReleaseSource)
			fakeS3BuiltReleaseSource = new(fetcherFakes.ReleaseSource)

			fetchExecuteArgs = []string{
				"--releases-directory", someReleasesDirectory,
			}
			releaseSourcesFactory = new(fakes.ReleaseSourcesFactory)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		JustBeforeEach(func() {
			fakeReleaseSources = []fetcher.ReleaseSource{fakeS3CompiledReleaseSource, fakeBoshIOReleaseSource, fakeS3BuiltReleaseSource}
			releaseSourcesFactory.ReleaseSourcesReturns(fakeReleaseSources)

			fetch = NewFetch(logger, kilnfile, kilnfileLock, releaseSourcesFactory, fakeLocalReleaseDirectory)

			fetchExecuteErr = fetch.Execute(fetchExecuteArgs)
		})

		// When a local compiled release exists
		//  When the releases' stemcell is different from the stemcell criteria
		//    It will return an error

		When("a local compiled release exists", func() {
			const (
				expectedStemcellOS      = "fooOS"
				expectedStemcellVersion = "0.2.0"
			)
			var (
				releaseID                               fetcher.ReleaseID
				releaseOnDisk                           fetcher.CompiledRelease
				actualStemcellOS, actualStemcellVersion string
			)
			BeforeEach(func() {
				releaseID = fetcher.ReleaseID{Name: "some-release", Version: "0.1.0"}
				fakeS3CompiledReleaseSource.GetMatchedReleasesReturns([]fetcher.RemoteRelease{
					fetcher.CompiledRelease{ID: releaseID, StemcellOS: expectedStemcellOS, StemcellVersion: expectedStemcellVersion},
				}, nil)
				fakeS3CompiledReleaseSource.DownloadReleasesReturns(
					fetcher.LocalReleaseSet{
						releaseID: fetcher.CompiledRelease{
							ID:              releaseID,
							StemcellOS:      expectedStemcellOS,
							StemcellVersion: expectedStemcellVersion,
							Path:            fmt.Sprintf("releases/%s-%s-%s-%s.tgz", releaseID.Name, releaseID.Version, expectedStemcellOS, expectedStemcellVersion),
						},
					}, nil)
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.Release{{Name: releaseID.Name, Version: releaseID.Version}},
					Stemcell: cargo.Stemcell{OS: expectedStemcellOS, Version: expectedStemcellVersion},
				}
				fetchExecuteArgs = append(fetchExecuteArgs, "--no-confirm")
			})

			When("the release was compiled with a different os", func() {
				BeforeEach(func() {
					releaseOnDisk = fetcher.CompiledRelease{
						ID:              releaseID,
						StemcellOS:      "different-os",
						StemcellVersion: expectedStemcellVersion,
						Path:            fmt.Sprintf("releases/%s-%s-%s-%s.tgz", releaseID.Name, releaseID.Version, actualStemcellOS, actualStemcellVersion),
					}
					fakeLocalReleaseDirectory.GetLocalReleasesReturns(
						fetcher.LocalReleaseSet{releaseID: releaseOnDisk},
						nil)
				})

				It("deletes the file from disk", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveLen(1))
					Expect(extras).To(ConsistOf(releaseOnDisk))
				})
			})

			When("the release was compiled with a different version of the same os", func() {
				BeforeEach(func() {
					releaseOnDisk = fetcher.CompiledRelease{
						ID:              releaseID,
						StemcellOS:      expectedStemcellOS,
						StemcellVersion: "404",
						Path:            fmt.Sprintf("releases/%s-%s-%s-%s.tgz", releaseID.Name, releaseID.Version, actualStemcellOS, actualStemcellVersion),
					}
					fakeLocalReleaseDirectory.GetLocalReleasesReturns(
						fetcher.LocalReleaseSet{releaseID: releaseOnDisk},
						nil)
				})

				It("deletes the file from disk", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(HaveLen(1))
					Expect(extras).To(ConsistOf(releaseOnDisk))
				})
			})
		})

		Context("starting with no releases and some are found in each release source (happy path)", func() {
			var (
				s3CompiledReleaseID = fetcher.ReleaseID{Name: "lts-compiled-release", Version: "1.2.4"}
				s3BuiltReleaseID    = fetcher.ReleaseID{Name: "lts-built-release", Version: "1.3.9"}
				boshIOReleaseID     = fetcher.ReleaseID{Name: "boshio-release", Version: "1.4.16"}
			)
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.Release{
						{Name: s3CompiledReleaseID.Name, Version: s3CompiledReleaseID.Version},
						{Name: s3BuiltReleaseID.Name, Version: s3BuiltReleaseID.Version},
						{Name: boshIOReleaseID.Name, Version: boshIOReleaseID.Version},
					},
					Stemcell: cargo.Stemcell{OS: "some-os", Version: "30.1"},
				}
				fakeS3CompiledReleaseSource.GetMatchedReleasesReturns(
					[]fetcher.RemoteRelease{
						fetcher.CompiledRelease{ID: s3CompiledReleaseID, StemcellOS: "some-os", StemcellVersion: "30.1", Path: "some-s3-key"},
					},
					nil)
				fakeS3CompiledReleaseSource.DownloadReleasesReturns(
					fetcher.LocalReleaseSet{
						s3CompiledReleaseID: fetcher.CompiledRelease{
							ID: s3CompiledReleaseID, StemcellOS: "some-os", StemcellVersion: "30.1", Path: "local-path",
						},
					},
					nil)

				fakeS3BuiltReleaseSource.GetMatchedReleasesReturns(
					[]fetcher.RemoteRelease{fetcher.BuiltRelease{ID: s3BuiltReleaseID, Path: "some-other-s3-key"}},
					nil)
				fakeS3BuiltReleaseSource.DownloadReleasesReturns(
					fetcher.LocalReleaseSet{
						s3BuiltReleaseID: fetcher.BuiltRelease{ID: s3BuiltReleaseID, Path: "some-other-s3-key"},
					},
					nil)

				fakeBoshIOReleaseSource.GetMatchedReleasesReturns(
					[]fetcher.RemoteRelease{fetcher.BuiltRelease{ID: boshIOReleaseID, Path: "some-bosh-io-url"}},
					nil)
				fakeBoshIOReleaseSource.DownloadReleasesReturns(
					fetcher.LocalReleaseSet{
						boshIOReleaseID: fetcher.BuiltRelease{ID: boshIOReleaseID, Path: "some-bosh-io-url"},
					},
					nil)

				fakeLocalReleaseDirectory.GetLocalReleasesReturns(fetcher.LocalReleaseSet{}, nil)
			})

			It("completes successfully", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())
			})

			It("fetches compiled release from s3 compiled release source", func() {
				Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))

				releasesDir, objects, threads := fakeS3CompiledReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(objects).To(ConsistOf(
					fetcher.CompiledRelease{
						ID:              s3CompiledReleaseID,
						StemcellOS:      "some-os",
						StemcellVersion: "30.1",
						Path:            "some-s3-key",
					}))
			})

			It("fetches built release from s3 built release source", func() {
				Expect(fakeS3BuiltReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				releasesDir, objects, threads := fakeS3BuiltReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(objects).To(ConsistOf(
					fetcher.BuiltRelease{
						ID:   s3BuiltReleaseID,
						Path: "some-other-s3-key",
					}))
			})

			It("fetches bosh.io release from bosh.io release source", func() {
				Expect(fakeBoshIOReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				releasesDir, objects, threads := fakeBoshIOReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(releasesDir).To(Equal(someReleasesDirectory))
				Expect(threads).To(Equal(0))
				Expect(objects).To(ConsistOf(
					fetcher.BuiltRelease{
						ID:   boshIOReleaseID,
						Path: "some-bosh-io-url",
					}))
			})
		})

		Context("when one or more releases are not available from release sources", func() {
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.Release{{Name: "not-found-in-any-release-source", Version: "0.0.1"}},
					Stemcell: cargo.Stemcell{OS: "some-os", Version: "30.1"},
				}
				fakeS3CompiledReleaseSource.GetMatchedReleasesReturns(nil, nil)
				fakeS3BuiltReleaseSource.GetMatchedReleasesReturns(nil, nil)
				fakeBoshIOReleaseSource.GetMatchedReleasesReturns(nil, nil)
			})

			It("reports an error", func() {
				err := fetch.Execute([]string{"--releases-directory", someReleasesDirectory})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					"could not find the following releases\n- not-found-in-any-release-source (0.0.1)")) // Could not find an exact match for these releases in any of the release sources we checked
			})
		})

		Context("when all releases are already present in output directory", func() {
			BeforeEach(func() {
				someLocalReleaseID := fetcher.ReleaseID{
					Name:    "some-release-from-local-dir",
					Version: "1.2.3",
				}
				expectedStemcell := cargo.Stemcell{OS: "some-os", Version: "4.5.6"}
				fakeLocalReleaseDirectory.GetLocalReleasesReturns(fetcher.LocalReleaseSet{
					someLocalReleaseID: fetcher.CompiledRelease{
						ID:              someLocalReleaseID,
						StemcellOS:      expectedStemcell.OS,
						StemcellVersion: expectedStemcell.Version,
						Path:            "/path/to/some/release",
					},
				}, nil)

				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.Release{{Name: someLocalReleaseID.Name, Version: someLocalReleaseID.Version}},
					Stemcell: expectedStemcell,
				}
			})

			It("no-ops", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
				Expect(fakeS3BuiltReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
				Expect(fakeBoshIOReleaseSource.DownloadReleasesCallCount()).To(Equal(0))
			})
		})

		Context("when some releases are already present in output directory", func() {
			var (
				missingReleaseS3CompiledID   fetcher.ReleaseID
				missingReleaseS3CompiledPath = "s3-key-some-missing-release-on-s3-compiled"
				missingReleaseBoshIOID       fetcher.ReleaseID
				missingReleaseBoshIOPath     = "some-other-bosh-io-key"
				missingReleaseS3BuiltID      fetcher.ReleaseID
				missingReleaseS3BuiltPath    = "s3-key-some-missing-release-on-s3-built"

				missingReleaseS3Compiled fetcher.CompiledRelease
				missingReleaseBoshIO,
				missingReleaseS3Built    fetcher.BuiltRelease
			)
			BeforeEach(func() {
				localRelease1ID := fetcher.ReleaseID{Name: "some-release", Version: "1.2.3"}
				localRelease2ID := fetcher.ReleaseID{Name: "some-tiny-release", Version: "1.2.3"}
				missingReleaseS3CompiledID = fetcher.ReleaseID{Name: "some-missing-release-on-s3-compiled", Version: "4.5.6"}
				missingReleaseBoshIOID = fetcher.ReleaseID{Name: "some-missing-release-on-boshio", Version: "5.6.7"}
				missingReleaseS3BuiltID = fetcher.ReleaseID{Name: "some-missing-release-on-s3-built", Version: "8.9.0"}

				expectedStemcell := cargo.Stemcell{OS: "some-os", Version: "4.5.6"}

				missingReleaseS3Compiled = fetcher.CompiledRelease{
					ID:              missingReleaseS3CompiledID,
					StemcellOS:      expectedStemcell.OS,
					StemcellVersion: expectedStemcell.Version,
					Path:            missingReleaseS3CompiledPath,
				}
				missingReleaseBoshIO = fetcher.BuiltRelease{ID: missingReleaseBoshIOID, Path: missingReleaseBoshIOPath}
				missingReleaseS3Built = fetcher.BuiltRelease{ID: missingReleaseS3BuiltID, Path: missingReleaseS3BuiltPath}

				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.Release{
						{Name: localRelease1ID.Name, Version: localRelease1ID.Version},
						{Name: localRelease2ID.Name, Version: localRelease2ID.Version},
						{Name: missingReleaseS3CompiledID.Name, Version: missingReleaseS3CompiledID.Version},
						{Name: missingReleaseBoshIOID.Name, Version: missingReleaseBoshIOID.Version},
						{Name: missingReleaseS3BuiltID.Name, Version: missingReleaseS3BuiltID.Version},
					},
					Stemcell: expectedStemcell,
				}

				fakeLocalReleaseDirectory.GetLocalReleasesReturns(fetcher.LocalReleaseSet{
					localRelease1ID: fetcher.CompiledRelease{
						ID:              localRelease1ID,
						StemcellOS:      expectedStemcell.OS,
						StemcellVersion: expectedStemcell.Version,
						Path:            "path/to/some/release",
					},
					// a release that has no compiled packages, such as consul-drain, will also have no stemcell criteria in release.MF.
					// we must make sure that we can match this kind of release properly to avoid unnecessary downloads.
					localRelease2ID: fetcher.BuiltRelease{ID: localRelease2ID, Path: "path/to/some/tiny/release"},
				}, nil)

				fakeS3CompiledReleaseSource.GetMatchedReleasesReturns([]fetcher.RemoteRelease{missingReleaseS3Compiled}, nil)
				fakeS3CompiledReleaseSource.DownloadReleasesReturns(fetcher.LocalReleaseSet{missingReleaseS3CompiledID: missingReleaseS3Compiled}, nil)

				fakeBoshIOReleaseSource.GetMatchedReleasesReturns([]fetcher.RemoteRelease{missingReleaseBoshIO}, nil)
				fakeBoshIOReleaseSource.DownloadReleasesReturns(fetcher.LocalReleaseSet{missingReleaseBoshIOID: missingReleaseBoshIO}, nil)

				fakeS3BuiltReleaseSource.GetMatchedReleasesReturns([]fetcher.RemoteRelease{missingReleaseS3Built}, nil)
				fakeS3BuiltReleaseSource.DownloadReleasesReturns(fetcher.LocalReleaseSet{missingReleaseS3BuiltID: missingReleaseS3Built}, nil)
			})

			It("downloads only the missing releases", func() {
				Expect(fetchExecuteErr).NotTo(HaveOccurred())

				Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				_, objects, _ := fakeS3CompiledReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(ConsistOf(missingReleaseS3Compiled))

				Expect(fakeBoshIOReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				_, objects, _ = fakeBoshIOReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(ConsistOf(missingReleaseBoshIO))

				Expect(fakeS3BuiltReleaseSource.DownloadReleasesCallCount()).To(Equal(1))
				_, objects, _ = fakeS3BuiltReleaseSource.DownloadReleasesArgsForCall(0)
				Expect(objects).To(HaveLen(1))
				Expect(objects).To(ConsistOf(missingReleaseS3Built))
			})

			Context("when download fails", func() {
				BeforeEach(func() {
					fakeS3CompiledReleaseSource.DownloadReleasesReturns(
						nil,
						errors.New("download failed"),
					)
				})

				It("returns an error", func() {
					Expect(fetchExecuteErr).To(HaveOccurred())
				})
			})
		})

		Context("when there are extra releases locally that are not in the Kilnfile.lock", func() {
			var (
				boshIOReleaseID = fetcher.ReleaseID{Name: "some-release", Version: "1.2.3"}
				localReleaseID  = fetcher.ReleaseID{Name: "some-extra-release", Version: "1.2.3"}
			)
			BeforeEach(func() {
				expectedStemcell := cargo.Stemcell{OS: "some-os", Version: "4.5.6"}

				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.Release{{Name: boshIOReleaseID.Name, Version: boshIOReleaseID.Version}},
					Stemcell: expectedStemcell,
				}

				fakeLocalReleaseDirectory.GetLocalReleasesReturns(fetcher.LocalReleaseSet{
					localReleaseID: fetcher.CompiledRelease{
						ID:              localReleaseID,
						StemcellOS:      expectedStemcell.OS,
						StemcellVersion: expectedStemcell.Version,
						Path:            "path/to/some/extra/release",
					},
				}, nil)

				fakeBoshIOReleaseSource.GetMatchedReleasesReturns(
					[]fetcher.RemoteRelease{fetcher.BuiltRelease{ID: boshIOReleaseID, Path: "some-bosh-io-url"}},
					nil)
				fakeBoshIOReleaseSource.DownloadReleasesReturns(
					fetcher.LocalReleaseSet{boshIOReleaseID: fetcher.BuiltRelease{ID: boshIOReleaseID, Path: "some-bosh-io-url"}},
					nil)

			})

			Context("in non-interactive mode", func() {
				BeforeEach(func() {
					fetchExecuteArgs = []string{
						"--releases-directory", someReleasesDirectory,
						"--no-confirm",
					}
				})

				It("deletes the extra releases", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.DownloadReleasesCallCount()).To(Equal(1))

					Expect(fakeLocalReleaseDirectory.DeleteExtraReleasesCallCount()).To(Equal(1))
					extras, noConfirm := fakeLocalReleaseDirectory.DeleteExtraReleasesArgsForCall(0)
					Expect(extras).To(HaveLen(1))
					Expect(noConfirm).To(Equal(true))
					Expect(extras).To(ConsistOf(
						fetcher.CompiledRelease{
							ID: fetcher.ReleaseID{
								Name:    "some-extra-release",
								Version: "1.2.3",
							},
							StemcellOS:      "some-os",
							StemcellVersion: "4.5.6",
							Path:            "path/to/some/extra/release",
						}))
				})
			})

			Context("when multiple variable files are provided", func() {
				BeforeEach(func() {
					kilnfile = cargo.Kilnfile{
						ReleaseSources: []cargo.ReleaseSourceConfig{
							{
								Type: "s3",
								Compiled: true,
								Bucket: "my-releases",
								AccessKeyId:     "newkey",
								SecretAccessKey: "newsecret",
								Regex:           `^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`,
							},
						},
					}

					fetchExecuteArgs = []string{
						"--releases-directory", someReleasesDirectory,
					}
				})

				It("interpolates variables from both files", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())

					Expect(fakeS3CompiledReleaseSource.GetMatchedReleasesCallCount()).To(Equal(1))
					_, _ = fakeS3CompiledReleaseSource.GetMatchedReleasesArgsForCall(0)
				})
			})

			Context("when # of download threads is specified", func() {
				BeforeEach(func() {
					fetchExecuteArgs = []string{
						"--releases-directory", someReleasesDirectory,
						"--download-threads", "10",
					}
				})

				It("passes concurrency parameter to DownloadReleases", func() {
					Expect(fetchExecuteErr).NotTo(HaveOccurred())
					_, _, threads := fakeS3CompiledReleaseSource.DownloadReleasesArgsForCall(0)
					Expect(threads).To(Equal(10))
				})
			})

			Context("failure cases", func() {
				Context("# of download threads is not a number", func() {
					It("returns an error", func() {
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
							"--download-threads", "not-a-number",
						})
						Expect(err).To(MatchError(fmt.Sprintf("invalid value \"not-a-number\" for flag -download-threads: parse error")))
					})
				})

				Context("when local releases cannot be accessed", func() {
					BeforeEach(func() {
						fakeLocalReleaseDirectory.GetLocalReleasesReturns(nil, errors.New("some-error"))
					})
					It("returns an error", func() {
						err := fetch.Execute([]string{
							"--releases-directory", someReleasesDirectory,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("some-error"))
					})
				})
			})
		})

	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			Expect(fetch.Usage()).To(Equal(jhanda.Usage{
				Description:      "Fetches releases listed in Kilnfile.lock from S3 and downloads it locally",
				ShortDescription: "fetches releases",
				Flags:            fetch.Options,
			}))
		})
	})
})
