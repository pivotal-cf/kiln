package commands_test

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/onsi/gomega/gbytes"

	"github.com/charlievieth/fs"
	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"gopkg.in/src-d/go-billy.v4/osfs"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"github.com/pivotal-cf/kiln/fetcher"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
)

var _ = Describe("CompileBuiltReleases", func() {
	const (
		stemcellOS       = "plan9"
		stemcellVersion  = "42"
		builtSourceID    = "built"
		compiledSourceID = "compiled"
	)

	var (
		builtReleaseSource    *fetcherFakes.ReleaseSource
		compiledReleaseSource *fetcherFakes.ReleaseSource
		kilnfile              cargo.Kilnfile
		kilnfileLock          cargo.KilnfileLock
		boshDirector          *fakes.BoshDirector
		boshDeployment        *fakes.BoshDeployment

		kilnfileLoader             *fakes.KilnfileLoader
		multiReleaseSourceProvider *fakes.MultiReleaseSourceProvider
		releaseUploaderFinder      *fakes.ReleaseUploaderFinder
		releaseUploader            *fetcherFakes.ReleaseUploader

		kilnfilePath string

		logger *log.Logger
		logBuf *gbytes.Buffer

		releasesPath, stemcellPath, stemcellSHA1 string

		command CompileBuiltReleases
	)

	blobIDContents := func(blobID string) string {
		return "contents of " + blobID
	}

	BeforeEach(func() {
		builtReleaseSource = new(fetcherFakes.ReleaseSource)
		builtReleaseSource.IDReturns(builtSourceID)
		compiledReleaseSource = new(fetcherFakes.ReleaseSource)
		compiledReleaseSource.IDReturns(compiledSourceID)

		kilnfileLoader = new(fakes.KilnfileLoader)
		kilnfile = cargo.Kilnfile{
			ReleaseSources: []cargo.ReleaseSourceConfig{
				{Type: "s3", Bucket: compiledSourceID, Publishable: true},
				{Type: "s3", Bucket: builtSourceID},
			},
		}
		kilnfileLock = cargo.KilnfileLock{
			Releases: []cargo.ReleaseLock{
				{Name: "uaa", Version: "1.2.3", RemoteSource: builtSourceID, RemotePath: "/remote/path/uaa-1.2.3.tgz", SHA1: "original-sha"},
				{Name: "capi", Version: "2.3.4", RemoteSource: builtSourceID, RemotePath: "/remote/path/capi-2.3.4.tgz", SHA1: "original-sha"},
				{Name: "bpm", Version: "1.6", RemoteSource: compiledSourceID, RemotePath: "not-used", SHA1: "original-sha"},
			},
			Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
		}

		multiReleaseSourceProvider = new(fakes.MultiReleaseSourceProvider)
		multiReleaseSourceProvider.Returns(fetcher.NewMultiReleaseSource(compiledReleaseSource, builtReleaseSource))

		releaseUploader = new(fetcherFakes.ReleaseUploader)
		releaseUploaderFinder = new(fakes.ReleaseUploaderFinder)
		releaseUploaderFinder.Returns(releaseUploader, nil)

		releaseUploader.UploadReleaseCalls(func(requirement release.Requirement, reader io.Reader) (remote release.Remote, err error) {
			return release.Remote{
				ID:         release.ID{Name: requirement.Name, Version: requirement.Version},
				RemotePath: fmt.Sprintf("%s/%s-%s-%s-%s.tgz", requirement.Name, requirement.Name, requirement.Version, requirement.StemcellOS, requirement.StemcellVersion),
				SourceID:   compiledSourceID,
			}, nil
		})

		tmpDir, err := ioutil.TempDir("", "compile-built-releases")
		Expect(err).NotTo(HaveOccurred())

		releasesPath = filepath.Join(tmpDir, "my-releases")
		stemcellPath = filepath.Join(tmpDir, "my-stemcell.tgz")
		kilnfilePath = filepath.Join(tmpDir, "Kilnfile")

		Expect(
			os.MkdirAll(releasesPath, 0755),
		).To(Succeed())

		stemcellSHA1, err = test_helpers.WriteStemcellTarball(stemcellPath, stemcellOS, stemcellVersion, osfs.New(""))
		Expect(err).NotTo(HaveOccurred())

		builtReleaseSource.DownloadReleaseCalls(func(releaseDir string, remote release.Remote, threads int) (release.Local, error) {
			localPath := filepath.Join(releaseDir, fmt.Sprintf("%s-%s.tgz", remote.Name, remote.Version))

			f, err := fs.Create(localPath)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			f.Write([]byte("file contents"))

			return release.Local{
				ID:        remote.ID,
				LocalPath: localPath,
				SHA1:      "not-used",
			}, nil
		})

		boshDirector = new(fakes.BoshDirector)
		boshDeployment = new(fakes.BoshDeployment)

		boshDirector.UploadStemcellFileReturns(nil)
		boshDirector.UploadReleaseFileReturns(nil)
		boshDirector.FindDeploymentReturns(boshDeployment, nil)
		boshDeployment.UpdateReturns(nil)

		boshDeployment.ExportReleaseCalls(func(releaseSlug boshdir.ReleaseSlug, _ boshdir.OSVersionSlug, _ []string) (boshdir.ExportReleaseResult, error) {
			blobID := fmt.Sprintf("%s-%s", releaseSlug.Name(), releaseSlug.Version())
			s := sha1.New()
			io.Copy(s, strings.NewReader(blobIDContents(blobID)))
			sha1 := hex.EncodeToString(s.Sum(nil))

			return boshdir.ExportReleaseResult{
				BlobstoreID: blobID,
				SHA1:        sha1,
			}, nil
		})
		boshDirector.DownloadResourceUncheckedCalls(func(blobID string, writer io.Writer) error {
			_, err := writer.Write([]byte(blobIDContents(blobID)))
			return err
		})

		logBuf = gbytes.NewBuffer()
		logger = log.New(logBuf, "", 0)
		command = CompileBuiltReleases{
			KilnfileLoader:             kilnfileLoader,
			MultiReleaseSourceProvider: multiReleaseSourceProvider.Spy,
			ReleaseUploaderFinder:      releaseUploaderFinder.Spy,
			BoshDirectorFactory:        func() (BoshDirector, error) { return boshDirector, nil },
			Logger:                     logger,
		}
	})

	JustBeforeEach(func() {
		kilnfileLoader.LoadKilnfilesReturns(kilnfile, kilnfileLock, nil)
	})

	When("everything succeeds (happy path)", func() {
		It("downloads the releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(compiledReleaseSource.DownloadReleaseCallCount()).To(Equal(0))

			Expect(builtReleaseSource.DownloadReleaseCallCount()).To(Equal(2))

			downloadDir, remote, threads := builtReleaseSource.DownloadReleaseArgsForCall(0)
			Expect(downloadDir).To(Equal(releasesPath))
			Expect(remote).To(Equal(release.Remote{
				ID:         release.ID{Name: "uaa", Version: "1.2.3"},
				RemotePath: "/remote/path/uaa-1.2.3.tgz",
				SourceID:   builtSourceID,
			}))
			Expect(threads).To(Equal(0))

			downloadDir, remote, threads = builtReleaseSource.DownloadReleaseArgsForCall(1)
			Expect(downloadDir).To(Equal(releasesPath))
			Expect(remote).To(Equal(release.Remote{
				ID:         release.ID{Name: "capi", Version: "2.3.4"},
				RemotePath: "/remote/path/capi-2.3.4.tgz",
				SourceID:   builtSourceID,
			}))
			Expect(threads).To(Equal(0))

			Expect(logBuf.Contents()).To(ContainSubstring(""))
		})

		It("compiles and downloads the compiled releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Uploading a release stemcell")
			Expect(boshDirector.UploadStemcellFileCallCount()).To(Equal(1))

			stemcellFile, fix := boshDirector.UploadStemcellFileArgsForCall(0)
			Expect(fix).To(BeFalse())

			_, err = stemcellFile.(*os.File).Seek(0, 0)
			Expect(err).NotTo(HaveOccurred())

			s := sha1.New()
			defer stemcellFile.Close()

			io.Copy(s, stemcellFile)
			actualStemcellSHA1 := hex.EncodeToString(s.Sum(nil))
			Expect(actualStemcellSHA1).To(Equal(stemcellSHA1))

			By("Uploading the releases")
			Expect(boshDirector.UploadReleaseFileCallCount()).To(Equal(2))

			uaaReleaseFile, rebase, fix := boshDirector.UploadReleaseFileArgsForCall(0)
			Expect(fix).To(BeFalse())
			Expect(rebase).To(BeFalse())

			uaaReleaseFileStats, err := uaaReleaseFile.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(uaaReleaseFileStats.Name()).To(Equal("uaa-1.2.3.tgz"))

			capiReleaseFile, rebase, fix := boshDirector.UploadReleaseFileArgsForCall(1)
			Expect(fix).To(BeFalse())
			Expect(rebase).To(BeFalse())

			capiReleaseFileStats, err := capiReleaseFile.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(capiReleaseFileStats.Name()).To(Equal("capi-2.3.4.tgz"))

			By("Creating a deployment for release compilation")
			Expect(boshDirector.FindDeploymentCallCount()).To(Equal(1))

			deploymentName := boshDirector.FindDeploymentArgsForCall(0)
			Expect(deploymentName).To(MatchRegexp("^compile-built-releases-([[:alnum:]]+)"))

			Expect(boshDeployment.UpdateCallCount()).To(Equal(1))
			manifest, options := boshDeployment.UpdateArgsForCall(0)
			Expect(options).To(BeZero())
			Expect(string(manifest)).To(MatchRegexp(`name: uaa\n\s+version: 1\.2\.3`))
			Expect(string(manifest)).To(MatchRegexp(`name: capi\n\s+version: 2\.3\.4`))
			Expect(string(manifest)).To(MatchRegexp(`alias: default\n\s+os: plan9\n\s+version: "42"`))

			By("exporting the compiled releases")
			Expect(boshDeployment.ExportReleaseCallCount()).To(Equal(2))
			releaseSlug, osVersionSlug, jobs := boshDeployment.ExportReleaseArgsForCall(0)
			Expect(releaseSlug.String()).To(Equal("uaa/1.2.3"))
			Expect(osVersionSlug.String()).To(Equal("plan9/42"))
			Expect(jobs).To(BeEmpty())

			releaseSlug, osVersionSlug, jobs = boshDeployment.ExportReleaseArgsForCall(1)
			Expect(releaseSlug.String()).To(Equal("capi/2.3.4"))
			Expect(osVersionSlug.String()).To(Equal("plan9/42"))
			Expect(jobs).To(BeEmpty())

			By("Downloading the underlying exported release from bosh")
			Expect(boshDirector.DownloadResourceUncheckedCallCount()).To(Equal(2))

			blobstoreID, filehandler := boshDirector.DownloadResourceUncheckedArgsForCall(0)
			Expect(blobstoreID).To(Equal("uaa-1.2.3"))
			Expect(filehandler.(*os.File).Name()).To(ContainSubstring("uaa-1.2.3-plan9-42.tgz"))

			blobstoreID, filehandler = boshDirector.DownloadResourceUncheckedArgsForCall(1)
			Expect(blobstoreID).To(Equal("capi-2.3.4"))
			Expect(filehandler.(*os.File).Name()).To(ContainSubstring("capi-2.3.4-plan9-42.tgz"))

			By("Cleaning up the deployment")
			Expect(boshDeployment.DeleteCallCount()).To(Equal(1))
			force := boshDeployment.DeleteArgsForCall(0)
			Expect(force).To(BeTrue())

			Expect(boshDirector.CleanUpCallCount()).To(Equal(1))
			all, dryRun, keepOrphanedDisks := boshDirector.CleanUpArgsForCall(0)
			Expect(all).To(BeTrue())
			Expect(dryRun).To(BeFalse())
			Expect(keepOrphanedDisks).To(BeFalse())
		})

		It("uploads the compiled releases to the release source", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseUploaderFinder.CallCount()).To(Equal(1))

			actualKilnfile, sourceID := releaseUploaderFinder.ArgsForCall(0)
			Expect(sourceID).To(Equal(compiledSourceID))
			Expect(actualKilnfile).To(Equal(kilnfile))

			Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(2))

			spec, releaseFile := releaseUploader.UploadReleaseArgsForCall(0)
			Expect(spec).To(Equal(release.Requirement{
				Name:            "uaa",
				Version:         "1.2.3",
				StemcellOS:      "plan9",
				StemcellVersion: "42",
			}))
			contents, err := ioutil.ReadAll(releaseFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("contents of uaa-1.2.3"))

			spec, releaseFile = releaseUploader.UploadReleaseArgsForCall(1)
			Expect(spec).To(Equal(release.Requirement{
				Name:            "capi",
				Version:         "2.3.4",
				StemcellOS:      "plan9",
				StemcellVersion: "42",
			}))
			contents, err = ioutil.ReadAll(releaseFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("contents of capi-2.3.4"))
		})

		It("updates the Kilnfile.lock with the compiled releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

			_, path, updatedLockfile := kilnfileLoader.SaveKilnfileLockArgsForCall(0)
			Expect(path).To(Equal(kilnfilePath))

			s := sha1.New()
			io.Copy(s, strings.NewReader(blobIDContents("uaa-1.2.3")))
			expectedUaaSha := hex.EncodeToString(s.Sum(nil))

			s = sha1.New()
			io.Copy(s, strings.NewReader(blobIDContents("capi-2.3.4")))
			expectedCapiSha := hex.EncodeToString(s.Sum(nil))

			Expect(updatedLockfile).To(Equal(cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         "uaa",
						Version:      "1.2.3",
						RemoteSource: compiledSourceID,
						RemotePath:   fmt.Sprintf("uaa/uaa-1.2.3-%s-%s.tgz", stemcellOS, stemcellVersion),
						SHA1:         expectedUaaSha,
					},
					{
						Name:         "capi",
						Version:      "2.3.4",
						RemoteSource: compiledSourceID,
						RemotePath:   fmt.Sprintf("capi/capi-2.3.4-%s-%s.tgz", stemcellOS, stemcellVersion),
						SHA1:         expectedCapiSha,
					},
					{
						Name:         "bpm",
						Version:      "1.6",
						RemoteSource: compiledSourceID,
						RemotePath:   "not-used",
						SHA1:         "original-sha",
					},
				},
				Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
			}))
		})
	})

	When("exporting the release fails", func() {
		const errorMsg = "absolutely no exportation >:("

		BeforeEach(func() {
			boshDeployment.ExportReleaseReturns(boshdir.ExportReleaseResult{}, errors.New(errorMsg))
		})

		It("still deletes the compilation deployment and cleans up director assets", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			Expect(err).To(MatchError(ContainSubstring("uaa")))

			Expect(boshDeployment.DeleteCallCount()).To(Equal(1))
			force := boshDeployment.DeleteArgsForCall(0)
			Expect(force).To(BeTrue())

			Expect(boshDirector.CleanUpCallCount()).To(Equal(1))
			all, dryRun, keepOrphanedDisks := boshDirector.CleanUpArgsForCall(0)
			Expect(all).To(BeTrue())
			Expect(dryRun).To(BeFalse())
			Expect(keepOrphanedDisks).To(BeFalse())
		})
	})

	When("downloading the exported release fails", func() {
		const errorMsg = "absolutely no downloading >:("

		BeforeEach(func() {
			boshDirector.DownloadResourceUncheckedReturns(errors.New(errorMsg))
		})

		It("still deletes the compilation deployment and cleans up director assets", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			Expect(err).To(MatchError(ContainSubstring("uaa")))

			Expect(boshDeployment.DeleteCallCount()).To(Equal(1))
			force := boshDeployment.DeleteArgsForCall(0)
			Expect(force).To(BeTrue())

			Expect(boshDirector.CleanUpCallCount()).To(Equal(1))
			all, dryRun, keepOrphanedDisks := boshDirector.CleanUpArgsForCall(0)
			Expect(all).To(BeTrue())
			Expect(dryRun).To(BeFalse())
			Expect(keepOrphanedDisks).To(BeFalse())
		})
	})

	When("deleting the deployment fails", func() {
		BeforeEach(func() {
			boshDeployment.DeleteReturns(errors.New("panic now >:DDD"))
		})

		It("panics", func() {
			Expect(func() {
				command.Execute([]string{
					"--kilnfile", kilnfilePath,
					"--releases-directory", releasesPath,
					"--stemcell-file", stemcellPath,
					"--upload-target-id", compiledSourceID,
				})
			}).To(Panic())
		})
	})

	When("cleaning up the director fails", func() {
		const errorMsg = "Keep the dirty bits!!"
		BeforeEach(func() {
			boshDirector.CleanUpReturns(boshdir.CleanUp{}, errors.New(errorMsg))
		})

		It("provides a warning", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(logBuf.Contents()).To(ContainSubstring(errorMsg))
		})
	})

	When("the lockfile refers to a non-existent release source", func() {
		BeforeEach(func() {
			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{Name: "uaa", Version: "1.2.3", RemoteSource: "no-such-source", RemotePath: "/remote/path/uaa-1.2.3.tgz", SHA1: "not-used"},
					{Name: "bpm", Version: "1.6", RemoteSource: compiledSourceID, RemotePath: "not-used", SHA1: "not-used"},
				},
				Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
			}
		})

		It("errors", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring("unknown release source")))
			Expect(err).To(MatchError(ContainSubstring("no-such-source")))
		})
	})

	When("a downloaded compiled release has an incorrect SHA", func() {
		BeforeEach(func() {
			boshDeployment.ExportReleaseReturns(boshdir.ExportReleaseResult{
				BlobstoreID: "my-blob",
				SHA1:        "aa64cc884828ae6e8f3d1a24f889e5b43843981f",
			}, nil)
		})

		It("errors", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring("incorrect SHA")))
			Expect(err).To(MatchError(ContainSubstring("uaa")))
		})
	})
})
