package commands_test

import (
	"errors"
	"log"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	fetcherFakes "github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
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
			releaseName           = "some-release-2"
			releaseOldVersion     = "42"
			releaseNewVersion     = "43"
			releaseOldSha         = "old-sha-2"
			releaseNewSha         = "new-sha-2"
			releaseOldSourceID    = "old-source-2"
			releaseOldRemotePath  = "old-path-2"
			releaseNewRemotePath  = "new-path-2"

			kilnfilePath     = "Kilnfile"
			kilnfileLockPath = kilnfilePath + ".lock"
		)

		var (
			syncWithLocal         commands.SyncWithLocal
			localReleaseDirectory *commandsFakes.LocalReleaseDirectory
			remotePatherFinder    *commandsFakes.RemotePatherFinder
			remotePather          *fetcherFakes.RemotePather
			fs                    billy.Filesystem
			kilnfileLock          cargo.KilnfileLock
		)

		BeforeEach(func() {
			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.BOSHReleaseLock{
					{
						Name:         release1Name,
						Version:      release1OldVersion,
						RemoteSource: release1OldSourceID,
						RemotePath:   release1OldRemotePath,
						SHA1:         release1OldSha,
					},
					{
						Name:         releaseName,
						Version:      releaseOldVersion,
						RemoteSource: releaseOldSourceID,
						RemotePath:   releaseOldRemotePath,
						SHA1:         releaseOldSha,
					},
				},
				Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
			}

			localReleaseDirectory = new(commandsFakes.LocalReleaseDirectory)
			localReleaseDirectory.GetLocalReleasesReturns([]component.Local{
				{
					Lock:      cargo.BOSHReleaseLock{Name: release1Name, Version: release1NewVersion, SHA1: release1NewSha},
					LocalPath: "local-path",
				},
				{
					Lock:      cargo.BOSHReleaseLock{Name: releaseName, Version: releaseNewVersion, SHA1: releaseNewSha},
					LocalPath: "local-path-2",
				},
			}, nil)

			remotePatherFinder = new(commandsFakes.RemotePatherFinder)
			remotePather = new(fetcherFakes.RemotePather)

			remotePatherFinder.Returns(remotePather, nil)
			remotePather.RemotePathCalls(func(requirement cargo.BOSHReleaseSpecification) (path string, err error) {
				switch requirement.Name {
				case release1Name:
					return release1NewRemotePath, nil
				case releaseName:
					return releaseNewRemotePath, nil
				default:
					panic("unexpected release name")
				}
			})

			fs = memfs.New()
			logger := log.New(GinkgoWriter, "", 0)

			syncWithLocal = commands.NewSyncWithLocal(fs, localReleaseDirectory, remotePatherFinder.Spy, logger)
		})

		JustBeforeEach(func() {
			Expect(fsWriteYAML(fs, kilnfileLockPath, kilnfileLock)).NotTo(HaveOccurred())
			Expect(fsWriteYAML(fs, kilnfilePath, cargo.Kilnfile{})).NotTo(HaveOccurred())
		})

		It("updates the Kilnfile.lock to have the same version as the local releases", func() {
			err := syncWithLocal.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--assume-release-source", releaseSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			var updatedLockfile cargo.KilnfileLock
			Expect(fsReadYAML(fs, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
			Expect(updatedLockfile.Releases).To(Equal([]cargo.BOSHReleaseLock{
				{
					Name:         release1Name,
					Version:      release1NewVersion,
					RemoteSource: releaseSourceID,
					RemotePath:   release1NewRemotePath,
					SHA1:         release1NewSha,
				},
				{
					Name:         releaseName,
					Version:      releaseNewVersion,
					RemoteSource: releaseSourceID,
					RemotePath:   releaseNewRemotePath,
					SHA1:         releaseNewSha,
				},
			}))
		})

		When("one of the releases on disk is the same version as in the Kilnfile.lock", func() {
			BeforeEach(func() {
				localReleaseDirectory.GetLocalReleasesReturns([]component.Local{
					{
						Lock:      cargo.BOSHReleaseLock{Name: release1Name, Version: release1OldVersion, SHA1: release1NewSha},
						LocalPath: "local-path",
					},
					{
						Lock:      cargo.BOSHReleaseLock{Name: releaseName, Version: releaseNewVersion, SHA1: releaseNewSha},
						LocalPath: "local-path-2",
					},
				}, nil)
			})

			It("updates the Kilnfile.lock to have the correct remote info and SHA1", func() {
				err := syncWithLocal.Execute([]string{
					"--kilnfile", kilnfilePath,
					"--assume-release-source", releaseSourceID,
				})
				Expect(err).NotTo(HaveOccurred())

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(fs, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile.Releases).To(Equal([]cargo.BOSHReleaseLock{
					{
						Name:         release1Name,
						Version:      release1OldVersion,
						RemoteSource: releaseSourceID,
						RemotePath:   release1NewRemotePath,
						SHA1:         release1NewSha,
					},
					{
						Name:         releaseName,
						Version:      releaseNewVersion,
						RemoteSource: releaseSourceID,
						RemotePath:   releaseNewRemotePath,
						SHA1:         releaseNewSha,
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

					var updatedLockfile cargo.KilnfileLock
					Expect(fsReadYAML(fs, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
					Expect(updatedLockfile.Releases).To(Equal([]cargo.BOSHReleaseLock{
						{
							Name:         release1Name,
							Version:      release1OldVersion,
							RemoteSource: release1OldSourceID,
							RemotePath:   release1OldRemotePath,
							SHA1:         release1OldSha,
						},
						{
							Name:         releaseName,
							Version:      releaseNewVersion,
							RemoteSource: releaseSourceID,
							RemotePath:   releaseNewRemotePath,
							SHA1:         releaseNewSha,
						},
					}))
				})
			})
		})

		When("a release on disk doesn't exist in the Kilnfile.lock", func() {
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.BOSHReleaseLock{
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
				Expect(err).To(MatchError(ContainSubstring(releaseName)))
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
