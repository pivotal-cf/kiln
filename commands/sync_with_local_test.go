package commands_test

import (
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"log"
)

var _ = Describe("sync-with-local", func() {
	Describe("Execute", func() {
		const (
			releaseSourceID       = "some-source"
			stemcellOS            = "linux-os"
			stemcellVersion       = "2.2"
			release1Name          = "some-release"
			release1OldVersion    = "1"
			release1NewVersion    = "2"
			release1OldSha        = "old-sha"
			release1NewSha        = "new-sha"
			release1OldSourceID   = "old-source"
			release1OldRemotePath = "old-path"
			release1NewRemotePath = "new-path"
			release2Name          = "some-release-2"
			release2OldVersion    = "42"
			release2NewVersion    = "43"
			release2OldSha        = "old-sha-2"
			release2NewSha        = "new-sha-2"
			release2OldSourceID   = "old-source-2"
			release2OldRemotePath = "old-path-2"
			release2NewRemotePath = "new-path-2"

			kilnfilePath = "Kilnfile"
		)

		var (
			syncWithLocal         SyncWithLocal
			kilnfileLoader        *fakes.KilnfileLoader
			localReleaseDirectory *fakes.LocalReleaseDirectory
			remotePatherFinder    *fakes.RemotePatherFinder
			remotePather          *fetcherFakes.RemotePather
			fs                    billy.Filesystem
			kilnfileLock          cargo.KilnfileLock
		)

		BeforeEach(func() {
			kilnfileLoader = new(fakes.KilnfileLoader)
			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         release1Name,
						Version:      release1OldVersion,
						RemoteSource: release1OldSourceID,
						RemotePath:   release1OldRemotePath,
						SHA1:         release1OldSha,
					},
					{
						Name:         release2Name,
						Version:      release2OldVersion,
						RemoteSource: release2OldSourceID,
						RemotePath:   release2OldRemotePath,
						SHA1:         release2OldSha,
					},
				},
				Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
			}

			localReleaseDirectory = new(fakes.LocalReleaseDirectory)
			localReleaseDirectory.GetLocalReleasesReturns([]release.Local{
				{
					ID:        release.ID{Name: release1Name, Version: release1NewVersion},
					LocalPath: "local-path",
					SHA1:      release1NewSha,
				},
				{
					ID:        release.ID{Name: release2Name, Version: release2NewVersion},
					LocalPath: "local-path-2",
					SHA1:      release2NewSha,
				},
			}, nil)

			remotePatherFinder = new(fakes.RemotePatherFinder)
			remotePather = new(fetcherFakes.RemotePather)

			remotePatherFinder.Returns(remotePather, nil)
			remotePather.RemotePathCalls(func(requirement release.Requirement) (path string, err error) {
				switch requirement.Name {
				case release1Name:
					return release1NewRemotePath, nil
				case release2Name:
					return release2NewRemotePath, nil
				default:
					panic("unexpected release name")
				}
			})

			fs = memfs.New()
			logger := log.New(GinkgoWriter, "", 0)

			syncWithLocal = NewSyncWithLocal(kilnfileLoader, fs, localReleaseDirectory, remotePatherFinder.Spy, logger)
		})

		JustBeforeEach(func() {
			kilnfileLoader.LoadKilnfilesReturns(cargo.Kilnfile{}, kilnfileLock, nil)
		})

		It("updates the Kilnfile.lock to have the same version as the local releases", func() {
			err := syncWithLocal.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--assume-release-source", releaseSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

			filesystem, path, updatedLockfile := kilnfileLoader.SaveKilnfileLockArgsForCall(0)
			Expect(filesystem).To(Equal(fs))
			Expect(path).To(Equal(kilnfilePath))
			Expect(updatedLockfile.Releases).To(Equal([]cargo.ReleaseLock{
				{
					Name:         release1Name,
					Version:      release1NewVersion,
					RemoteSource: releaseSourceID,
					RemotePath:   release1NewRemotePath,
					SHA1:         release1NewSha,
				},
				{
					Name:         release2Name,
					Version:      release2NewVersion,
					RemoteSource: releaseSourceID,
					RemotePath:   release2NewRemotePath,
					SHA1:         release2NewSha,
				},
			}))
		})

		When("one of the releases on disk is the same version as in the Kilnfile.lock", func() {
			BeforeEach(func() {
				localReleaseDirectory.GetLocalReleasesReturns([]release.Local{
					{
						ID:        release.ID{Name: release1Name, Version: release1OldVersion},
						LocalPath: "local-path",
						SHA1:      release1NewSha,
					},
					{
						ID:        release.ID{Name: release2Name, Version: release2NewVersion},
						LocalPath: "local-path-2",
						SHA1:      release2NewSha,
					},
				}, nil)
			})

			It("updates the Kilnfile.lock to have the correct remote info and SHA1", func() {
				err := syncWithLocal.Execute([]string{
					"--kilnfile", kilnfilePath,
					"--assume-release-source", releaseSourceID,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

				filesystem, path, updatedLockfile := kilnfileLoader.SaveKilnfileLockArgsForCall(0)
				Expect(filesystem).To(Equal(fs))
				Expect(path).To(Equal(kilnfilePath))
				Expect(updatedLockfile.Releases).To(Equal([]cargo.ReleaseLock{
					{
						Name:         release1Name,
						Version:      release1OldVersion,
						RemoteSource: releaseSourceID,
						RemotePath:   release1NewRemotePath,
						SHA1:         release1NewSha,
					},
					{
						Name:         release2Name,
						Version:      release2NewVersion,
						RemoteSource: releaseSourceID,
						RemotePath:   release2NewRemotePath,
						SHA1:         release2NewSha,
					},
				}))
			})

			When("--skip-same-version is passed", func() {
				It("doesn't modify that entry", func() {
					err := syncWithLocal.Execute([]string{
						"--kilnfile", kilnfilePath,
						"--assume-release-source", releaseSourceID,
						"--skip-same-version",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

					filesystem, path, updatedLockfile := kilnfileLoader.SaveKilnfileLockArgsForCall(0)
					Expect(filesystem).To(Equal(fs))
					Expect(path).To(Equal(kilnfilePath))
					Expect(updatedLockfile.Releases).To(Equal([]cargo.ReleaseLock{
						{
							Name:         release1Name,
							Version:      release1OldVersion,
							RemoteSource: release1OldSourceID,
							RemotePath:   release1OldRemotePath,
							SHA1:         release1OldSha,
						},
						{
							Name:         release2Name,
							Version:      release2NewVersion,
							RemoteSource: releaseSourceID,
							RemotePath:   release2NewRemotePath,
							SHA1:         release2NewSha,
						},
					}))
				})
			})
		})

		When("a release on disk doesn't exist in the Kilnfile.lock", func() {
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name:         release1Name,
							Version:      release1OldVersion,
							RemoteSource: release1OldSourceID,
							RemotePath:   release1OldRemotePath,
							SHA1:         release1OldSha,
						},
					},
					Stemcell: cargo.Stemcell{},
				}
			})

			It("returns an error", func() {
				err := syncWithLocal.Execute([]string{
					"--kilnfile", kilnfilePath,
					"--assume-release-source", releaseSourceID,
				})

				Expect(err).To(MatchError(ContainSubstring("does not exist")))
				Expect(err).To(MatchError(ContainSubstring(release2Name)))
			})
		})

		When("there's an error generating the remote path for a release", func() {
			BeforeEach(func() {
				remotePather.RemotePathReturns("", errors.New("bad bad stuff"))
			})

			It("returns an error", func() {
				err := syncWithLocal.Execute([]string{
					"--kilnfile", kilnfilePath,
					"--assume-release-source", releaseSourceID,
				})

				Expect(err).To(MatchError(ContainSubstring("bad bad stuff")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
			})
		})
	})
})
