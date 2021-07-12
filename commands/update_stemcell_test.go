package commands_test

import (
	"errors"
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	release2 "github.com/pivotal-cf/kiln/pkg/release"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"gopkg.in/src-d/go-billy.v4/osfs"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	. "github.com/pivotal-cf/kiln/commands"
)

var _ = Describe("UpdateStemcell", func() {
	var _ jhanda.Command = UpdateStemcell{}

	const (
		newStemcellOS      = "some-os"
		newStemcellVersion = "1.2.3"

		release1Name    = "release1"
		release1Version = "1"
		release2Name    = "release2"
		release2Version = "2"

		newRelease1SHA        = "new-sha1-1"
		newRelease1RemotePath = "new-remote-path-1"
		newRelease2SHA        = "new-sha1-2"
		newRelease2RemotePath = "new-remote-path-2"

		publishableReleaseSourceID   = "publishable"
		unpublishableReleaseSourceID = "test-only"

		releasesDirPath = "releases-dir"
	)

	Describe("Execute", func() {
		var (
			update                             *UpdateStemcell
			tmpDir, kilnfilePath, stemcellPath string
			kilnfileLoader                     *fakes.KilnfileLoader
			kilnfile                           cargo2.Kilnfile
			kilnfileLock                       cargo2.KilnfileLock
			releaseSource                      *fetcherFakes.MultiReleaseSource
			outputBuffer                       *gbytes.Buffer
		)

		BeforeEach(func() {
			var err error

			kilnfileLoader = new(fakes.KilnfileLoader)
			kilnfile = cargo2.Kilnfile{}
			kilnfileLock = cargo2.KilnfileLock{
				Releases: []cargo2.ReleaseLock{
					{
						Name:         release1Name,
						SHA1:         "old-sha-1",
						Version:      release1Version,
						RemoteSource: "old-remote-source-1",
						RemotePath:   "old-remote-path-1",
					},
					{
						Name:         release2Name,
						SHA1:         "old-sha-2",
						Version:      release2Version,
						RemoteSource: "old-remote-source-2",
						RemotePath:   "old-remote-path-2",
					},
				},
				Stemcell: cargo2.Stemcell{
					OS:      "old-os",
					Version: "0.1",
				},
			}

			releaseSource = new(fetcherFakes.MultiReleaseSource)
			releaseSource.GetMatchedReleaseCalls(func(requirement release2.Requirement) (release2.Remote, bool, error) {
				switch requirement.Name {
				case release1Name:
					remote := release2.Remote{
						ID:         release2.ID{Name: release1Name, Version: release1Version},
						RemotePath: newRelease1RemotePath,
						SourceID:   publishableReleaseSourceID,
					}
					return remote, true, nil
				case release2Name:
					remote := release2.Remote{
						ID:         release2.ID{Name: release2Name, Version: release2Version},
						RemotePath: newRelease2RemotePath,
						SourceID:   unpublishableReleaseSourceID,
					}
					return remote, true, nil
				default:
					panic("unexpected release name")
				}
			})

			releaseSource.DownloadReleaseCalls(func(_ string, remote release2.Remote, _ int) (release2.Local, error) {
				switch remote.Name {
				case release1Name:
					local := release2.Local{
						ID:        release2.ID{Name: release1Name, Version: release1Version},
						LocalPath: "not-used",
						SHA1:      newRelease1SHA,
					}
					return local, nil
				case release2Name:
					local := release2.Local{
						ID:        release2.ID{Name: release2Name, Version: release2Version},
						LocalPath: "not-used",
						SHA1:      newRelease2SHA,
					}
					return local, nil
				default:
					panic("unexpected release name")
				}
			})

			multiReleaseSourceProvider := new(fakes.MultiReleaseSourceProvider)
			multiReleaseSourceProvider.Returns(releaseSource)

			tmpDir, err = ioutil.TempDir("", "fetch-test")
			Expect(err).NotTo(HaveOccurred())

			kilnfilePath = filepath.Join(tmpDir, "Kilnfile")

			stemcellPath = filepath.Join(tmpDir, "my-stemcell.tgz")
			test_helpers.WriteStemcellTarball(stemcellPath, newStemcellOS, newStemcellVersion, osfs.New(""))

			outputBuffer = gbytes.NewBuffer()
			logger := log.New(outputBuffer, "", 0)

			update = &UpdateStemcell{
				KilnfileLoader:             kilnfileLoader,
				MultiReleaseSourceProvider: multiReleaseSourceProvider.Spy,
				Logger:                     logger,
			}
		})

		JustBeforeEach(func() {
			kilnfileLoader.LoadKilnfilesReturns(kilnfile, kilnfileLock, nil)
		})

		AfterEach(func() {
			Expect(
				os.RemoveAll(tmpDir),
			).To(Succeed())
		})

		It("updates the Kilnfile.lock contents", func() {
			err := update.Execute([]string{"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath})
			Expect(err).NotTo(HaveOccurred())

			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

			_, path, updatedLockfile := kilnfileLoader.SaveKilnfileLockArgsForCall(0)
			Expect(path).To(Equal(kilnfilePath))

			Expect(updatedLockfile.Stemcell).To(Equal(cargo2.Stemcell{
				OS:      newStemcellOS,
				Version: newStemcellVersion,
			}))

			Expect(updatedLockfile.Releases).To(Equal([]cargo2.ReleaseLock{
				{
					Name:         release1Name,
					Version:      release1Version,
					SHA1:         newRelease1SHA,
					RemoteSource: publishableReleaseSourceID,
					RemotePath:   newRelease1RemotePath,
				},
				{
					Name:         release2Name,
					Version:      release2Version,
					SHA1:         newRelease2SHA,
					RemoteSource: unpublishableReleaseSourceID,
					RemotePath:   newRelease2RemotePath,
				},
			}))
		})

		It("looks up the correct releases", func() {
			err := update.Execute([]string{
				"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath, "--releases-directory", releasesDirPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(2))

			req1 := releaseSource.GetMatchedReleaseArgsForCall(0)
			Expect(req1).To(Equal(release2.Requirement{
				Name: release1Name, Version: release1Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
			}))

			req2 := releaseSource.GetMatchedReleaseArgsForCall(1)
			Expect(req2).To(Equal(release2.Requirement{
				Name: release2Name, Version: release2Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
			}))
		})

		It("downloads the correct releases", func() {
			err := update.Execute([]string{
				"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath, "--releases-directory", releasesDirPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(2))

			actualDir, remote1, threads := releaseSource.DownloadReleaseArgsForCall(0)
			Expect(actualDir).To(Equal(releasesDirPath))
			Expect(remote1).To(Equal(
				release2.Remote{
					ID:         release2.ID{Name: release1Name, Version: release1Version},
					RemotePath: newRelease1RemotePath,
					SourceID:   publishableReleaseSourceID,
				},
			))
			Expect(threads).To(Equal(0))

			actualDir, remote2, threads := releaseSource.DownloadReleaseArgsForCall(1)
			Expect(actualDir).To(Equal(releasesDirPath))
			Expect(remote2).To(Equal(
				release2.Remote{
					ID:         release2.ID{Name: release2Name, Version: release2Version},
					RemotePath: newRelease2RemotePath,
					SourceID:   unpublishableReleaseSourceID,
				},
			))
			Expect(threads).To(Equal(0))
		})

		When("the stemcell didn't change", func() {
			BeforeEach(func() {
				kilnfileLock.Stemcell = cargo2.Stemcell{
					OS:      newStemcellOS,
					Version: newStemcellVersion,
				}
			})

			It("no-ops", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(0))
				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(0))
				Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(0))

				Expect(outputBuffer.Contents()).To(ContainSubstring("Nothing to update for product"))
			})
		})

		When("the remote information for a release doesn't change", func() {
			BeforeEach(func() {
				kilnfileLock.Releases[1].RemoteSource = unpublishableReleaseSourceID
				kilnfileLock.Releases[1].RemotePath = newRelease2RemotePath
			})

			It("doesn't download the release", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, remote, _ := releaseSource.DownloadReleaseArgsForCall(0)
				Expect(remote.Name).To(Equal(release1Name))

				Expect(string(outputBuffer.Contents())).To(ContainSubstring("No change"))
				Expect(string(outputBuffer.Contents())).To(ContainSubstring(release2Name))
			})
		})

		When("the release can't be found", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(release2.Remote{}, false, nil)
			})

			It("errors", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath})

				Expect(err).To(MatchError(ContainSubstring("couldn't find release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
			})
		})

		When("finding the release errors", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(release2.Remote{}, false, errors.New("big badda boom"))
			})

			It("errors", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath})

				Expect(err).To(MatchError(ContainSubstring("finding release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
				Expect(err).To(MatchError(ContainSubstring("big badda boom")))
			})
		})

		When("downloading the release errors", func() {
			BeforeEach(func() {
				releaseSource.DownloadReleaseReturns(release2.Local{}, errors.New("big badda boom"))
			})

			It("errors", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--stemcell-file", stemcellPath})

				Expect(err).To(MatchError(ContainSubstring("downloading release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
				Expect(err).To(MatchError(ContainSubstring("big badda boom")))
			})
		})
	})
})
