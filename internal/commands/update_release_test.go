package commands_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/onsi/gomega/gbytes"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	fetcherFakes "github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("UpdateRelease", func() {
	const (
		releaseName                    = "capi"
		oldReleaseVersion              = "1.8.0"
		newReleaseVersion              = "1.8.7"
		notDownloadedReleaseVersion    = "1.8.4"
		oldRemotePath                  = "https://bosh.io/releases/some-release"
		newRemotePath                  = "some/s3/path"
		notDownloadedRemotePath        = "some-other/s3/path"
		oldReleaseSourceName           = "bosh.io"
		newReleaseSourceName           = "final-pcf-bosh-releases"
		notDownloadedReleaseSourceName = "compiled-releases"
		oldReleaseSha1                 = "old-sha1"
		newReleaseSha1                 = "new-sha1"
		notDownloadedReleaseSha1       = "some-other-new-sha1"

		releasesDir = "releases"

		kilnfilePath     = "Kilnfile"
		kilnfileLockPath = kilnfilePath + ".lock"
	)

	var (
		updateReleaseCommand       commands.UpdateRelease
		filesystem                 billy.Filesystem
		multiReleaseSourceProvider *commandsFakes.MultiReleaseSourceProvider
		releaseSource              *fetcherFakes.MultiReleaseSource
		logger                     *log.Logger
		downloadedReleasePath      string
		expectedDownloadedRelease  component.Local
		expectedRemoteRelease      component.Lock
		kilnfileLock               cargo.KilnfileLock
	)

	Context("Execute", func() {
		BeforeEach(func() {
			releaseSource = new(fetcherFakes.MultiReleaseSource)
			multiReleaseSourceProvider = new(commandsFakes.MultiReleaseSourceProvider)
			multiReleaseSourceProvider.Returns(releaseSource)

			filesystem = osfs.New("/tmp/")

			kilnfile := cargo.Kilnfile{}

			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ComponentLock{
					{
						Name:    "minecraft",
						Version: "2.0.1",

						SHA1:         "developersdevelopersdevelopersdevelopers",
						RemoteSource: "bosh.io",
						RemotePath:   "not-used",
					},
					{
						Name:    releaseName,
						Version: oldReleaseVersion,

						SHA1:         oldReleaseSha1,
						RemoteSource: oldReleaseSourceName,
						RemotePath:   oldRemotePath,
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "some-os",
					Version: "4.5.6",
				},
			}

			Expect(fsWriteYAML(filesystem, kilnfilePath, kilnfile)).NotTo(HaveOccurred())
			Expect(fsWriteYAML(filesystem, kilnfileLockPath, kilnfileLock)).NotTo(HaveOccurred())

			logger = log.New(GinkgoWriter, "", 0)

			err := filesystem.MkdirAll(releasesDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			downloadedReleasePath = filepath.Join(releasesDir, fmt.Sprintf("%s-%s.tgz", releaseName, newReleaseVersion))
			expectedDownloadedRelease = component.Local{
				Lock:      component.Lock{Name: releaseName, Version: newReleaseVersion, SHA1: newReleaseSha1},
				LocalPath: downloadedReleasePath,
			}
			expectedRemoteRelease = expectedDownloadedRelease.Lock.WithRemote(newReleaseSourceName, newRemotePath)
			exepectedNotDownloadedRelease := component.Lock{
				Name:         releaseName,
				Version:      notDownloadedReleaseVersion,
				RemotePath:   notDownloadedRemotePath,
				RemoteSource: notDownloadedReleaseSourceName,
				SHA1:         notDownloadedReleaseSha1,
			}

			releaseSource.GetMatchedReleaseReturns(expectedRemoteRelease, true, nil)
			releaseSource.FindReleaseVersionReturns(exepectedNotDownloadedRelease, true, nil)
			releaseSource.DownloadReleaseReturns(expectedDownloadedRelease, nil)
		})

		JustBeforeEach(func() {
			updateReleaseCommand = commands.NewUpdateRelease(logger, filesystem, multiReleaseSourceProvider.Spy)
		})

		When("updating to a version that exists in the remote", func() {
			It("downloads the release", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(1))

				receivedReleaseRequirement := releaseSource.GetMatchedReleaseArgsForCall(0)
				releaseRequirement := component.Spec{
					Name:            releaseName,
					Version:         newReleaseVersion,
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}
				Expect(receivedReleaseRequirement).To(Equal(releaseRequirement))

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(1))

				receivedReleasesDir, receivedRemoteRelease := releaseSource.DownloadReleaseArgsForCall(0)
				Expect(receivedReleasesDir).To(Equal(releasesDir))
				Expect(receivedRemoteRelease).To(Equal(expectedRemoteRelease))
			})

			It("writes the new version to the Kilnfile.lock", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(filesystem, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile.Releases).To(HaveLen(2))
				Expect(updatedLockfile.Releases).To(ContainElement(
					cargo.ComponentLock{
						Name:    releaseName,
						Version: newReleaseVersion,

						SHA1:         newReleaseSha1,
						RemoteSource: newReleaseSourceName,
						RemotePath:   newRemotePath,
					},
				))
			})

			It("considers all release sources", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(multiReleaseSourceProvider.CallCount()).To(Equal(1))
				_, allowOnlyPublishable := multiReleaseSourceProvider.ArgsForCall(0)
				Expect(allowOnlyPublishable).To(BeFalse())
			})
		})

		When("passing the --allow-only-publishable-releases flag", func() {
			var downloadErr error

			BeforeEach(func() {
				downloadErr = errors.New("asplode!!")
				releaseSource.DownloadReleaseReturns(component.Local{}, downloadErr)
			})

			It("tells the release downloader factory to allow only publishable releases", func() {
				err := updateReleaseCommand.Execute([]string{
					"--allow-only-publishable-releases",
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(downloadErr))

				Expect(multiReleaseSourceProvider.CallCount()).To(Equal(1))
				_, allowOnlyPublishable := multiReleaseSourceProvider.ArgsForCall(0)
				Expect(allowOnlyPublishable).To(BeTrue())
			})
		})

		When("none of the release's fields change", func() {
			var logBuf *gbytes.Buffer

			BeforeEach(func() {
				expectedDownloadedRelease = component.Local{
					Lock:      component.Lock{Name: releaseName, Version: oldReleaseVersion, SHA1: oldReleaseSha1},
					LocalPath: "not-used",
				}
				expectedRemoteRelease = component.Lock{
					Name: releaseName, Version: oldReleaseVersion,
					RemotePath:   oldRemotePath,
					RemoteSource: oldReleaseSourceName,
				}

				releaseSource.GetMatchedReleaseReturns(expectedRemoteRelease, true, nil)
				releaseSource.DownloadReleaseReturns(expectedDownloadedRelease, nil)

				logBuf = gbytes.NewBuffer()
				logger = log.New(logBuf, "", 0)
			})

			It("doesn't update the Kilnfile.lock", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", oldReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(filesystem, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile).To(Equal(kilnfileLock))
			})

			It("notifies the user", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", oldReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(string(logBuf.Contents())).To(ContainSubstring("No changes made"))
				Expect(string(logBuf.Contents())).NotTo(ContainSubstring("Updated"))
				Expect(string(logBuf.Contents())).NotTo(ContainSubstring("COMMIT"))
			})
		})

		When("the named release isn't in Kilnfile.lock", func() {
			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("no release named \"no-such-release\"")))
				Expect(err).To(MatchError(ContainSubstring("try removing the -release")))
			})

			It("does not try to download anything", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(0))
				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(0))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(filesystem, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile).To(Equal(kilnfileLock))
			})
		})

		When("the release can't be found", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(component.Lock{}, false, errors.New("bad stuff"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("bad stuff")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(filesystem, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile).To(Equal(kilnfileLock))
			})
		})

		When("downloading the release fails", func() {
			BeforeEach(func() {
				releaseSource.DownloadReleaseReturns(component.Local{}, errors.New("bad stuff"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("bad stuff")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(filesystem, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile).To(Equal(kilnfileLock))
			})
		})

		When("invalid arguments are given", func() {
			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{"--no-such-flag"})
				Expect(err).To(MatchError(ContainSubstring("-no-such-flag")))
			})
		})

		When("updating lock file without downloading", func() {
			It("writes the new version to the Kilnfile.lock", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
					"--without-download",
				})
				Expect(err).NotTo(HaveOccurred())

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(filesystem, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile.Releases).To(HaveLen(2))
				Expect(updatedLockfile.Releases).To(ContainElement(
					cargo.ComponentLock{
						Name:         releaseName,
						Version:      notDownloadedReleaseVersion,
						SHA1:         notDownloadedReleaseSha1,
						RemoteSource: notDownloadedReleaseSourceName,
						RemotePath:   notDownloadedRemotePath,
					},
				))
			})
		})
	})
})
