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
	"gopkg.in/yaml.v2"
	"log"
)

var _ = Describe("sync-with-local", func() {
	Describe("Execute", func() {
		const (
			releaseSourceID    = "some-source"
			stemcellOS         = "linux-os"
			stemcellVersion    = "2.2"
			release1Name       = "some-release"
			release1Version    = "1"
			release1NewSha     = "new-sha"
			release1RemotePath = "new-path"
			release2Name       = "some-release-2"
			release2Version    = "2"
			release2NewSha     = "new-sha-2"
			release2RemotePath = "new-path-2"

			kilnfilePath = "Kilnfile"
		)

		var (
			syncWithLocal         SyncWithLocal
			kilnfileLoader        *fakes.KilnfileLoader
			localReleaseDirectory *fakes.LocalReleaseDirectory
			remotePatherFactory   *fakes.RemotePatherFactory
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
						Version:      release1Version,
						RemoteSource: "old-source",
						RemotePath:   "old-path",
						SHA1:         "old-sha",
					},
					{
						Name:         release2Name,
						Version:      release2Version,
						RemoteSource: "old-source-2",
						RemotePath:   "old-path-2",
						SHA1:         "old-sha-2",
					},
				},
				Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
			}

			localReleaseDirectory = new(fakes.LocalReleaseDirectory)
			localReleaseDirectory.GetLocalReleasesReturns([]release.Local{
				{
					ID:        release.ID{Name: release1Name, Version: release1Version},
					LocalPath: "local-path",
					SHA1:      release1NewSha,
				},
				{
					ID:        release.ID{Name: release2Name, Version: release2Version},
					LocalPath: "local-path-2",
					SHA1:      release2NewSha,
				},
			}, nil)

			remotePatherFactory = new(fakes.RemotePatherFactory)
			remotePather = new(fetcherFakes.RemotePather)

			remotePatherFactory.RemotePatherReturns(remotePather, nil)
			remotePather.RemotePathCalls(func(requirement release.Requirement) (path string, err error) {
				switch requirement.Name {
				case release1Name:
					return release1RemotePath, nil
				case release2Name:
					return release2RemotePath, nil
				default:
					panic("unexpected release name")
				}
			})

			fs = memfs.New()
			logger := log.New(GinkgoWriter, "", 0)

			syncWithLocal = NewSyncWithLocal(kilnfileLoader, fs, localReleaseDirectory, remotePatherFactory, logger)
		})

		JustBeforeEach(func() {
			kilnfileLoader.LoadKilnfilesReturns(cargo.Kilnfile{}, kilnfileLock, nil)
		})

		It("updates the Kilnfile.lock to have the same version as the local releases", func() {
			err := syncWithLocal.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--assume-release-source", releaseSourceID,
			})

			kilnfileLockFile, err := fs.Open(kilnfilePath + ".lock")
			Expect(err).NotTo(HaveOccurred())

			var kilnfileLockResult cargo.KilnfileLock
			err = yaml.NewDecoder(kilnfileLockFile).Decode(&kilnfileLockResult)
			Expect(err).NotTo(HaveOccurred())

			Expect(kilnfileLockResult.Releases).To(Equal([]cargo.ReleaseLock{
				{
					Name:         release1Name,
					Version:      release1Version,
					RemoteSource: releaseSourceID,
					RemotePath:   release1RemotePath,
					SHA1:         release1NewSha,
				},
				{
					Name:         release2Name,
					Version:      release2Version,
					RemoteSource: releaseSourceID,
					RemotePath:   release2RemotePath,
					SHA1:         release2NewSha,
				},
			}))
		})

		When("a release on disk doesn't exist in the Kilnfile.lock", func() {
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{
							Name:         release1Name,
							Version:      release1Version,
							RemoteSource: "old-source",
							RemotePath:   "old-path",
							SHA1:         "old-sha",
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
