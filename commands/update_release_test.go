package commands_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"
)

var _ = Describe("UpdateRelease", func() {
	const (
		releaseName       = "capi"
		releaseVersion    = "1.8.7"
		releasesDir       = "releases"
		remotePath        = "s3://pivotal"
		releaseSourceName = "LaBreaTarPit"
		releaseSha1       = "new-sha1"
	)

	var (
		updateReleaseCommand      UpdateRelease
		preexistingKilnfileLock   []byte
		filesystem                billy.Filesystem
		multiReleaseSourceProvider      *fakes.MultiReleaseSourceProvider
		releaseSource             *fetcherFakes.ReleaseSource
		logger                    *log.Logger
		downloadedReleasePath     string
		expectedDownloadedRelease release.Local
		expectedRemoteRelease     release.Remote
		kilnFileLoader            *fakes.KilnfileLoader
	)

	Context("Execute", func() {
		BeforeEach(func() {
			kilnFileLoader = new(fakes.KilnfileLoader)
			releaseSource = new(fetcherFakes.ReleaseSource)
			multiReleaseSourceProvider = new(fakes.MultiReleaseSourceProvider)
			multiReleaseSourceProvider.Returns(releaseSource)

			filesystem = osfs.New("/tmp/")

			kilnfile := cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{{Type: "bosh.io"}},
			}

			kilnFileLock := cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:    "minecraft",
						SHA1:    "developersdevelopersdevelopersdevelopers",
						Version: "2.0.1",
					},
					{
						Name:    releaseName,
						SHA1:    "old-sha1",
						Version: "1.87.0",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "some-os",
					Version: "4.5.6",
				},
			}

			kilnFileLoader.LoadKilnfilesReturns(kilnfile, kilnFileLock, nil)

			kfl, err := filesystem.Create("Kilnfile.lock")
			Expect(err).NotTo(HaveOccurred())
			defer kfl.Close()

			preexistingKilnfileLock, err = yaml.Marshal(kilnFileLock)
			_, err = kfl.Write(preexistingKilnfileLock)
			Expect(err).NotTo(HaveOccurred())

			logger = log.New(GinkgoWriter, "", 0)

			err = filesystem.MkdirAll(releasesDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			downloadedReleasePath = filepath.Join(releasesDir, fmt.Sprintf("%s-%s.tgz", releaseName, releaseVersion))
			expectedDownloadedRelease = release.Local{
				ID:        release.ID{Name: releaseName, Version: releaseVersion},
				LocalPath: downloadedReleasePath,
				SHA1:      releaseSha1,
			}
			expectedRemoteRelease = release.Remote{
				ID:         expectedDownloadedRelease.ID,
				RemotePath: remotePath,
				SourceID:   releaseSourceName,
			}

			releaseSource.GetMatchedReleaseReturns(expectedRemoteRelease, true, nil)
			releaseSource.DownloadReleaseReturns(expectedDownloadedRelease, nil)
		})

		JustBeforeEach(func() {
			updateReleaseCommand = NewUpdateRelease(logger, filesystem, multiReleaseSourceProvider.Spy, kilnFileLoader)
		})

		When("updating to a version that exists in the remote", func() {
			It("downloads the release", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
					"--variable", "someKey=someValue",
					"--variables-file", "thisisafile",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(1))

				receivedReleaseRequirement := releaseSource.GetMatchedReleaseArgsForCall(0)
				releaseRequirement := release.Requirement{
					Name:            releaseName,
					Version:         releaseVersion,
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}
				Expect(receivedReleaseRequirement).To(Equal(releaseRequirement))

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(1))

				receivedReleasesDir, receivedRemoteRelease, _ := releaseSource.DownloadReleaseArgsForCall(0)
				Expect(receivedReleasesDir).To(Equal(releasesDir))
				Expect(receivedRemoteRelease).To(Equal(expectedRemoteRelease))

				Expect(kilnFileLoader.LoadKilnfilesCallCount()).To(Equal(1))
				fs, kilnfilePath, variablesFiles, variables := kilnFileLoader.LoadKilnfilesArgsForCall(0)

				Expect(fs).To(Equal(filesystem))
				Expect(kilnfilePath).To(Equal("Kilnfile"))
				Expect(variablesFiles).To(Equal([]string{"thisisafile"}))
				Expect(variables).To(Equal([]string{"someKey=someValue"}))
			})

			It("writes the new version to the Kilnfile.lock", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				newKilnfileLock, err := filesystem.Open("Kilnfile.lock")
				Expect(err).NotTo(HaveOccurred())

				var kilnfileLock cargo.KilnfileLock
				err = yaml.NewDecoder(newKilnfileLock).Decode(&kilnfileLock)
				Expect(err).NotTo(HaveOccurred())
				Expect(kilnfileLock.Releases).To(HaveLen(2))
				Expect(kilnfileLock.Releases).To(ContainElement(
					cargo.ReleaseLock{
						Name:         releaseName,
						Version:      releaseVersion,
						SHA1:         releaseSha1,
						RemoteSource: releaseSourceName,
						RemotePath:   remotePath,
					},
				))
			})

			It("considers all release sources", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
					"--variable", "someKey=someValue",
					"--variables-file", "thisisafile",
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
				releaseSource.DownloadReleaseReturns(release.Local{}, downloadErr)
			})

			It("tells the release downloader factory to allow only publishable releases", func() {
				err := updateReleaseCommand.Execute([]string{
					"--allow-only-publishable-releases",
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
					"--variable", "someKey=someValue",
					"--variables-file", "thisisafile",
				})
				Expect(err).To(MatchError(downloadErr))

				Expect(multiReleaseSourceProvider.CallCount()).To(Equal(1))
				_, allowOnlyPublishable := multiReleaseSourceProvider.ArgsForCall(0)
				Expect(allowOnlyPublishable).To(BeTrue())
			})
		})

		When("the named release isn't in Kilnfile.lock", func() {
			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("no release named \"no-such-release\"")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})

				expectKilnfileLockIsUnchanged(filesystem, preexistingKilnfileLock)
			})
		})

		When("there is an error loading the Kilnfiles", func() {
			BeforeEach(func() {
				kilnFileLoader.LoadKilnfilesReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, errors.New("big bada boom"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("big bada boom")))
			})
		})

		When("the release can't be found", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(release.Remote{}, false, errors.New("bad stuff"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("bad stuff")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})

				expectKilnfileLockIsUnchanged(filesystem, preexistingKilnfileLock)
			})
		})

		When("downloading the release fails", func() {
			BeforeEach(func() {
				releaseSource.DownloadReleaseReturns(release.Local{}, errors.New("bad stuff"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("bad stuff")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})

				expectKilnfileLockIsUnchanged(filesystem, preexistingKilnfileLock)
			})
		})

		When("reopening the Kilnfile.lock fails", func() {
			var expectedError error

			BeforeEach(func() {
				expectedError = errors.New("very very bad")

				ogFilesystem := filesystem
				filesystem = fakeFilesystem{
					Filesystem: ogFilesystem,
					CreateFunc: func(path string) (billy.File, error) {
						return nil, expectedError
					},
				}
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring(expectedError.Error())))
			})
		})

		When("writing to the Kilnfile.lock fails", func() {
			var expectedError error

			BeforeEach(func() {
				expectedError = errors.New("i don't feel so good")

				badFile := unwritableFile{err: expectedError}
				ogFilesystem := filesystem
				filesystem = fakeFilesystem{
					Filesystem: ogFilesystem,
					CreateFunc: func(path string) (billy.File, error) { return badFile, nil },
				}
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", releaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring(expectedError.Error())))
			})
		})

		When("invalid arguments are given", func() {
			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{"--no-such-flag"})
				Expect(err).To(MatchError(ContainSubstring("-no-such-flag")))
			})
		})
	})
})

func expectKilnfileLockIsUnchanged(fs billy.Filesystem, originalContents []byte) {
	file, err := fs.Open("Kilnfile.lock")
	Expect(err).NotTo(HaveOccurred())

	contents, err := ioutil.ReadAll(file)
	Expect(err).NotTo(HaveOccurred())

	Expect(contents).To(BeEquivalentTo(originalContents))
}

type fakeFilesystem struct {
	billy.Filesystem
	CreateFunc func(string) (billy.File, error)
}

func (fs fakeFilesystem) Create(path string) (billy.File, error) {
	return fs.CreateFunc(path)
}

type unwritableFile struct {
	billy.File
	err error
}

func (f unwritableFile) Write(_ []byte) (int, error) {
	return 0, f.err
}
