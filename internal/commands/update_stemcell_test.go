package commands_test

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/jhanda"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	fetcherFakes "github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("UpdateStemcell", func() {
	var _ jhanda.Command = commands.UpdateStemcell{}

	const (
		newStemcellOS      = "old-os"
		newStemcellVersion = "1.100"

		release1Name    = "release1"
		release1Version = "1"
		release2Name    = "release2"
		release2Version = "2"
		release3Name    = "release3"
		release3Version = "3"

		newRelease1SHA        = "new-sha1-1"
		newRelease1RemotePath = "new-remote-path-1"
		newRelease2SHA        = "new-sha1-2"
		newRelease2RemotePath = "new-remote-path-2"
		newRelease3SHA        = "new-sha1-3"
		newRelease3RemotePath = "new-remote-path-3"

		publishableReleaseSourceID   = "publishable"
		unpublishableReleaseSourceID = "test-only"

		releasesDirPath = "releases-dir"
	)

	Describe("Execute", func() {
		var (
			update                                 *commands.UpdateStemcell
			tmpDir, kilnfilePath, kilnfileLockPath string
			fs                                     billy.Filesystem
			kilnfile                               cargo.Kilnfile
			kilnfileLock                           cargo.KilnfileLock
			releaseSource                          *fetcherFakes.MultiReleaseSource
			outputBuffer                           *gbytes.Buffer
		)

		BeforeEach(func() {
			fs = memfs.New()

			kilnfile = cargo.Kilnfile{
				Stemcell: cargo.Stemcell{
					OS:      "old-os",
					Version: "^1",
				},
				Releases: []cargo.BOSHReleaseTarballSpecification{
					{Name: release1Name, GitHubRepository: "https://example.com/lemon", Version: "*"},
					{Name: release2Name, GitHubRepository: "https://example.com/orange", Version: "*"},
					{Name: release3Name, GitHubRepository: "https://example.com/pomelo", Version: "*"},
				},
			}
			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.BOSHReleaseTarballLock{
					{
						Name:         release1Name,
						Version:      release1Version,
						SHA1:         "old-sha-1",
						RemoteSource: "old-remote-source-1",
						RemotePath:   "old-remote-path-1",
					},
					{
						Name:         release2Name,
						Version:      release2Version,
						SHA1:         "old-sha-2",
						RemoteSource: "old-remote-source-2",
						RemotePath:   "old-remote-path-2",
					},
					{
						Name:         release3Name,
						Version:      release3Version,
						SHA1:         "old-sha-3",
						RemoteSource: "old-remote-source-3",
						RemotePath:   "old-remote-path-3",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "old-os",
					Version: "1.1",
				},
			}

			releaseSource = new(fetcherFakes.MultiReleaseSource)
			releaseSource.GetMatchedReleaseCalls(func(requirement cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
				switch requirement.Name {
				case release1Name:
					remote := cargo.BOSHReleaseTarballLock{
						Name: release1Name, Version: release1Version,
						RemotePath:   newRelease1RemotePath,
						RemoteSource: publishableReleaseSourceID,
						SHA1:         "",
					}
					return remote, nil
				case release2Name:
					remote := cargo.BOSHReleaseTarballLock{
						Name: release2Name, Version: release2Version,
						RemotePath:   newRelease2RemotePath,
						RemoteSource: unpublishableReleaseSourceID,
						SHA1:         "not-calculated",
					}
					return remote, nil
				case release3Name:
					remote := cargo.BOSHReleaseTarballLock{
						Name: release3Name, Version: release3Version,
						RemotePath:   newRelease3RemotePath,
						RemoteSource: publishableReleaseSourceID,
						SHA1:         newRelease3SHA,
					}
					return remote, nil
				default:
					panic("unexpected release name")
				}
			})

			releaseSource.FindReleaseVersionCalls(func(requirement cargo.BOSHReleaseTarballSpecification, download bool) (cargo.BOSHReleaseTarballLock, error) {
				switch requirement.Name {
				case release1Name:
					remote := cargo.BOSHReleaseTarballLock{
						Name: release1Name, Version: release1Version,
						RemotePath:   newRelease1RemotePath,
						RemoteSource: publishableReleaseSourceID,
						SHA1:         "",
					}
					return remote, nil
				case release2Name:
					remote := cargo.BOSHReleaseTarballLock{
						Name: release2Name, Version: release2Version,
						RemotePath:   newRelease2RemotePath,
						RemoteSource: unpublishableReleaseSourceID,
						SHA1:         "not-calculated",
					}
					return remote, nil
				case release3Name:
					remote := cargo.BOSHReleaseTarballLock{
						Name: release3Name, Version: release3Version,
						RemotePath:   newRelease3RemotePath,
						RemoteSource: publishableReleaseSourceID,
						SHA1:         newRelease3SHA,
					}
					return remote, nil
				default:
					panic("unexpected release name")
				}
			})

			releaseSource.DownloadReleaseCalls(func(_ string, remote cargo.BOSHReleaseTarballLock) (component.Local, error) {
				switch remote.Name {
				case release1Name:
					local := component.Local{
						Lock:      cargo.BOSHReleaseTarballLock{Name: release1Name, Version: release1Version, SHA1: newRelease1SHA},
						LocalPath: "not-used",
					}
					return local, nil
				case release2Name:
					local := component.Local{
						Lock:      cargo.BOSHReleaseTarballLock{Name: release2Name, Version: release2Version, SHA1: newRelease2SHA},
						LocalPath: "not-used",
					}
					return local, nil
				default:
					panic("unexpected release name '" + remote.Name + "'")
				}
			})

			multiReleaseSourceProvider := new(fakes.MultiReleaseSourceProvider)
			multiReleaseSourceProvider.Returns(releaseSource)

			kilnfilePath = filepath.Join("Kilnfile")
			kilnfileLockPath = kilnfilePath + ".lock"

			outputBuffer = gbytes.NewBuffer()
			logger := log.New(outputBuffer, "", 0)

			update = &commands.UpdateStemcell{
				FS:                         fs,
				MultiReleaseSourceProvider: multiReleaseSourceProvider.Spy,
				Logger:                     logger,
			}
		})

		JustBeforeEach(func() {
			Expect(createYAMLFile(fs, kilnfilePath, kilnfile)).NotTo(HaveOccurred())
			Expect(createYAMLFile(fs, kilnfileLockPath, kilnfileLock)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(
				os.RemoveAll(tmpDir),
			).To(Succeed())
		})

		It("updates the Kilnfile.lock contents", func() {
			err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})
			Expect(err).NotTo(HaveOccurred())

			var updatedLockfile cargo.KilnfileLock
			Expect(fsReadYAML(fs, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
			Expect(updatedLockfile.Stemcell).To(Equal(cargo.Stemcell{
				OS:      newStemcellOS,
				Version: newStemcellVersion,
			}))
			Expect(updatedLockfile.Releases).To(HaveLen(3))
			Expect(updatedLockfile.Releases).To(ContainElement(
				cargo.BOSHReleaseTarballLock{
					Name:         release1Name,
					Version:      release1Version,
					SHA1:         newRelease1SHA,
					RemoteSource: publishableReleaseSourceID,
					RemotePath:   newRelease1RemotePath,
				},
			))
			Expect(updatedLockfile.Releases).To(ContainElement(
				cargo.BOSHReleaseTarballLock{
					Name:         release2Name,
					Version:      release2Version,
					SHA1:         newRelease2SHA,
					RemoteSource: unpublishableReleaseSourceID,
					RemotePath:   newRelease2RemotePath,
				},
			))
			Expect(updatedLockfile.Releases).To(ContainElement(
				cargo.BOSHReleaseTarballLock{
					Name:         release3Name,
					Version:      release3Version,
					SHA1:         newRelease3SHA,
					RemoteSource: publishableReleaseSourceID,
					RemotePath:   newRelease3RemotePath,
				},
			))
		})

		It("looks up the correct releases", func() {
			err := update.Execute([]string{
				"--kilnfile", kilnfilePath, "--version", "1.100", "--releases-directory", releasesDirPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(3))

			req1 := releaseSource.GetMatchedReleaseArgsForCall(0)
			Expect(req1).To(Equal(cargo.BOSHReleaseTarballSpecification{
				Name: release1Name, Version: release1Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
				GitHubRepository: "https://example.com/lemon",
			}))

			req2 := releaseSource.GetMatchedReleaseArgsForCall(1)
			Expect(req2).To(Equal(cargo.BOSHReleaseTarballSpecification{
				Name: release2Name, Version: release2Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
				GitHubRepository: "https://example.com/orange",
			}))

			req3 := releaseSource.GetMatchedReleaseArgsForCall(2)
			Expect(req3).To(Equal(cargo.BOSHReleaseTarballSpecification{
				Name: release3Name, Version: release3Version,
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
				GitHubRepository: "https://example.com/pomelo",
			}))
		})
		It("looks up the correct releases with --update-releases", func() {
			err := update.Execute([]string{
				"--kilnfile", kilnfilePath, "--version", "1.100", "--releases-directory", releasesDirPath, "--update-releases",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.FindReleaseVersionCallCount()).To(Equal(3))

			req1, noDownload1 := releaseSource.FindReleaseVersionArgsForCall(0)
			Expect(req1).To(Equal(cargo.BOSHReleaseTarballSpecification{
				Name: release1Name, Version: "*",
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
				GitHubRepository: "https://example.com/lemon",
			}))
			Expect(noDownload1).To(BeTrue())

			req2, noDownload2 := releaseSource.FindReleaseVersionArgsForCall(1)
			Expect(req2).To(Equal(cargo.BOSHReleaseTarballSpecification{
				Name: release2Name, Version: "*",
				StemcellOS: newStemcellOS, StemcellVersion: newStemcellVersion,
				GitHubRepository: "https://example.com/orange",
			}))
			Expect(noDownload2).To(BeTrue())
		})

		It("downloads 2 of the 3 correct releases, ", func() {
			err := update.Execute([]string{
				"--kilnfile", kilnfilePath, "--version", newStemcellVersion, "--releases-directory", releasesDirPath,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(2))

			actualDir, remote1 := releaseSource.DownloadReleaseArgsForCall(0)
			Expect(actualDir).To(Equal(releasesDirPath))
			Expect(remote1).To(Equal(
				cargo.BOSHReleaseTarballLock{
					Name: release1Name, Version: release1Version,
					RemotePath:   newRelease1RemotePath,
					RemoteSource: publishableReleaseSourceID,
					SHA1:         "",
				},
			))

			actualDir, remote2 := releaseSource.DownloadReleaseArgsForCall(1)
			Expect(actualDir).To(Equal(releasesDirPath))
			Expect(remote2).To(Equal(
				cargo.BOSHReleaseTarballLock{
					Name: release2Name, Version: release2Version,
					RemotePath:   newRelease2RemotePath,
					RemoteSource: unpublishableReleaseSourceID,
					SHA1:         "not-calculated",
				},
			))
		})

		When("the version input is invalid", func() {
			BeforeEach(func() {
				kilnfileLock.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: newStemcellVersion,
				}
			})

			It("no-ops", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", "34$5235.32235"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid stemcell version"))
			})
		})

		When("the kilnfile version constraint is invalid", func() {
			BeforeEach(func() {
				kilnfile.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: "$2353",
				}
			})

			It("no-ops", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", "2.100"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid stemcell constraint in kilnfile:"))
			})
		})

		When("the stemcell didn't change", func() {
			BeforeEach(func() {
				kilnfileLock.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: newStemcellVersion,
				}
			})

			It("no-ops", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(0))
				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(0))

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(fs, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile).To(Equal(kilnfileLock))

				Expect(outputBuffer.Contents()).To(ContainSubstring("Nothing to update for product"))
			})
		})

		When("the input stemcell version does not match kilnfile version constraint", func() {
			BeforeEach(func() {
				kilnfileLock.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: "~2019",
				}

				kilnfileLock.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: "2019.118",
				}
			})

			It("no-ops", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", "621.113"})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(0))
				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(0))

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile).To(Equal(kilnfileLock))

				Expect(string(outputBuffer.Contents())).To(ContainSubstring("Latest version does not satisfy the stemcell version constraint in kilnfile"))
			})
		})

		When("the kilnlockfile stemcell version is greater than input stemcell version", func() {
			BeforeEach(func() {
				kilnfileLock.Stemcell = cargo.Stemcell{
					OS:      newStemcellOS,
					Version: "1.102",
				}
			})

			It("allows downgrades and updates the Kilnfile.lock contents", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})
				Expect(err).NotTo(HaveOccurred())

				var updatedLockfile cargo.KilnfileLock
				Expect(fsReadYAML(fs, kilnfileLockPath, &updatedLockfile)).NotTo(HaveOccurred())
				Expect(updatedLockfile.Stemcell).To(Equal(cargo.Stemcell{
					OS:      newStemcellOS,
					Version: newStemcellVersion,
				}))

				Expect(updatedLockfile.Releases).To(HaveLen(3))
				Expect(updatedLockfile.Releases).To(ContainElement(
					cargo.BOSHReleaseTarballLock{
						Name:         release1Name,
						Version:      release1Version,
						SHA1:         newRelease1SHA,
						RemoteSource: publishableReleaseSourceID,
						RemotePath:   newRelease1RemotePath,
					},
				))
				Expect(updatedLockfile.Releases).To(ContainElement(
					cargo.BOSHReleaseTarballLock{
						Name:         release2Name,
						Version:      release2Version,
						SHA1:         newRelease2SHA,
						RemoteSource: unpublishableReleaseSourceID,
						RemotePath:   newRelease2RemotePath,
					},
				))
				Expect(updatedLockfile.Releases).To(ContainElement(
					cargo.BOSHReleaseTarballLock{
						Name:         release3Name,
						Version:      release3Version,
						SHA1:         newRelease3SHA,
						RemoteSource: publishableReleaseSourceID,
						RemotePath:   newRelease3RemotePath,
					},
				))
			})
		})

		When("the remote information for a release doesn't change", func() {
			BeforeEach(func() {
				kilnfileLock.Releases[1].RemoteSource = unpublishableReleaseSourceID
				kilnfileLock.Releases[1].RemotePath = newRelease2RemotePath
			})

			It("doesn't download the release", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(1))
				_, remote := releaseSource.DownloadReleaseArgsForCall(0)
				Expect(remote.Name).To(Equal(release1Name))

				Expect(string(outputBuffer.Contents())).To(ContainSubstring("No change"))
				Expect(string(outputBuffer.Contents())).To(ContainSubstring(release2Name))
			})
		})

		When("the release can't be found", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(cargo.BOSHReleaseTarballLock{}, component.ErrNotFound)
			})

			It("errors", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})

				Expect(err).To(MatchError(And(
					ContainSubstring(component.ErrNotFound.Error()),
					ContainSubstring(release1Name),
				)))
			})
		})

		When("finding the release errors", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(cargo.BOSHReleaseTarballLock{}, errors.New("big badda boom"))
			})

			It("errors", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})

				Expect(err).To(MatchError(ContainSubstring("finding release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
				Expect(err).To(MatchError(ContainSubstring("big badda boom")))
			})
		})

		When("downloading the release errors", func() {
			BeforeEach(func() {
				releaseSource.DownloadReleaseReturns(component.Local{}, errors.New("big badda boom"))
			})

			It("errors", func() {
				err := update.Execute([]string{"--kilnfile", kilnfilePath, "--version", newStemcellVersion})

				Expect(err).To(MatchError(ContainSubstring("downloading release")))
				Expect(err).To(MatchError(ContainSubstring(release1Name)))
				Expect(err).To(MatchError(ContainSubstring("big badda boom")))
			})
		})
	})
})

func createYAMLFile(fs billy.Filesystem, fp string, data any) error {
	f, err := fs.Create(fp)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	return yaml.NewEncoder(f).Encode(data)
}
