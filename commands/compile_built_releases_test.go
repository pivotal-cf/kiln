package commands_test

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
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
		boshDeployment1       *fakes.BoshDeployment
		boshDeployment2       *fakes.BoshDeployment
		boshDeployment3       *fakes.BoshDeployment

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
		builtReleaseSource.PublishableReturns(false)
		compiledReleaseSource = new(fetcherFakes.ReleaseSource)
		compiledReleaseSource.IDReturns(compiledSourceID)
		compiledReleaseSource.PublishableReturns(true)

		kilnfileLoader = new(fakes.KilnfileLoader)
		kilnfile = cargo.Kilnfile{
			ReleaseSources: []cargo.ReleaseSourceConfig{
				{Type: "s3", ID: compiledSourceID, Bucket: "not-used", Publishable: true},
				{Type: "s3", ID: builtSourceID, Bucket: "not-used-2"},
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
		multiReleaseSourceProvider.Calls(func(kilnfile cargo.Kilnfile, allowOnlyPublishable bool) fetcher.MultiReleaseSource {
			if allowOnlyPublishable {
				return fetcher.NewMultiReleaseSource(compiledReleaseSource)
			} else {
				return fetcher.NewMultiReleaseSource(compiledReleaseSource, builtReleaseSource)
			}
		})

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
		boshDeployment1 = new(fakes.BoshDeployment)
		boshDeployment1.NameReturns("deployment-1")
		boshDeployment2 = new(fakes.BoshDeployment)
		boshDeployment3 = new(fakes.BoshDeployment)

		boshDirector.UploadStemcellFileReturns(nil)
		boshDirector.UploadReleaseFileReturns(nil)
		boshDirector.FindDeploymentStub = func(deploymentName string) (boshdir.Deployment, error) {
			switch {
			case strings.HasPrefix(deploymentName, "compile-built-releases-0"):
				return boshDeployment1, nil
			case strings.HasPrefix(deploymentName, "compile-built-releases-1"):
				return boshDeployment2, nil
			case strings.HasPrefix(deploymentName, "compile-built-releases-2"):
				return boshDeployment3, nil
			default:
				return nil, errors.New("unknown-deployment, test setup is incorrect")
			}
		}
		boshDeployment1.UpdateReturns(nil)
		boshDeployment2.UpdateReturns(nil)
		boshDeployment3.UpdateReturns(nil)

		exportReleaseStub := func(releaseSlug boshdir.ReleaseSlug, _ boshdir.OSVersionSlug, _ []string) (boshdir.ExportReleaseResult, error) {
			blobID := fmt.Sprintf("%s-%s", releaseSlug.Name(), releaseSlug.Version())
			digest, err := boshcrypto.NewMultipleDigest(
				strings.NewReader(blobIDContents(blobID)),
				[]boshcrypto.Algorithm{boshcrypto.DigestAlgorithmSHA256},
			)
			Expect(err).NotTo(HaveOccurred())

			return boshdir.ExportReleaseResult{
				BlobstoreID: blobID,
				SHA1:        digest.String(),
			}, nil
		}

		boshDeployment1.ExportReleaseCalls(exportReleaseStub)
		boshDeployment2.ExportReleaseCalls(exportReleaseStub)
		boshDeployment3.ExportReleaseCalls(exportReleaseStub)

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

			Expect(boshDeployment1.UpdateCallCount()).To(Equal(1))
			manifest, options := boshDeployment1.UpdateArgsForCall(0)
			Expect(options).To(BeZero())
			Expect(string(manifest)).To(MatchRegexp(`name: uaa\n\s+version: 1\.2\.3`))
			Expect(string(manifest)).To(MatchRegexp(`name: capi\n\s+version: 2\.3\.4`))
			Expect(string(manifest)).To(MatchRegexp(`alias: default\n\s+os: plan9\n\s+version: "42"`))

			By("exporting the compiled releases")
			Expect(boshDeployment1.ExportReleaseCallCount()).To(Equal(2))
			var releaseSlugs []string
			releaseSlug, osVersionSlug, jobs := boshDeployment1.ExportReleaseArgsForCall(0)
			releaseSlugs = append(releaseSlugs, releaseSlug.String())
			Expect(osVersionSlug.String()).To(Equal("plan9/42"))
			Expect(jobs).To(BeEmpty())

			releaseSlug, osVersionSlug, jobs = boshDeployment1.ExportReleaseArgsForCall(1)
			releaseSlugs = append(releaseSlugs, releaseSlug.String())
			Expect(osVersionSlug.String()).To(Equal("plan9/42"))
			Expect(jobs).To(BeEmpty())

			Expect(releaseSlugs).To(ContainElements([]string{"uaa/1.2.3", "capi/2.3.4"}))

			By("Downloading the underlying exported release from bosh")
			Expect(boshDirector.DownloadResourceUncheckedCallCount()).To(Equal(2))

			var blobstoreIDs []string
			var filehandlerNames []string
			blobstoreID, filehandler := boshDirector.DownloadResourceUncheckedArgsForCall(0)
			blobstoreIDs = append(blobstoreIDs, blobstoreID)
			filehandlerNames = append(filehandlerNames, filehandler.(*os.File).Name())

			blobstoreID, filehandler = boshDirector.DownloadResourceUncheckedArgsForCall(1)
			blobstoreIDs = append(blobstoreIDs, blobstoreID)
			filehandlerNames = append(filehandlerNames, filehandler.(*os.File).Name())

			Expect(blobstoreIDs).To(ContainElements([]string{"uaa-1.2.3", "capi-2.3.4"}))
			Expect(filehandlerNames).To(ConsistOf(ContainSubstring("uaa-1.2.3-plan9-42.tgz"), ContainSubstring("capi-2.3.4-plan9-42.tgz")))

			By("Cleaning up the deployment")
			Expect(boshDeployment1.DeleteCallCount()).To(Equal(1))
			force := boshDeployment1.DeleteArgsForCall(0)
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

			spec1, releaseFile := releaseUploader.UploadReleaseArgsForCall(0)
			contents1, err := ioutil.ReadAll(releaseFile)
			Expect(err).NotTo(HaveOccurred())

			spec2, releaseFile := releaseUploader.UploadReleaseArgsForCall(1)
			contents2, err := ioutil.ReadAll(releaseFile)
			Expect(err).NotTo(HaveOccurred())

			Expect([]release.Requirement{spec1, spec2}).To(ConsistOf(
				release.Requirement{
					Name:            "uaa",
					Version:         "1.2.3",
					StemcellOS:      "plan9",
					StemcellVersion: "42",
				},
				release.Requirement{
					Name:            "capi",
					Version:         "2.3.4",
					StemcellOS:      "plan9",
					StemcellVersion: "42",
				},
			))

			Expect([]string{string(contents1), string(contents2)}).To(ConsistOf("contents of capi-2.3.4", "contents of uaa-1.2.3"))
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

		When("using parallel option", func() {
			BeforeEach(func() {
				kilnfileLock = cargo.KilnfileLock{
					Releases: []cargo.ReleaseLock{
						{Name: "uaa", Version: "1.2.3", RemoteSource: builtSourceID, RemotePath: "/remote/path/uaa-1.2.3.tgz", SHA1: "original-sha"},
						{Name: "capi", Version: "2.3.4", RemoteSource: builtSourceID, RemotePath: "/remote/path/capi-2.3.4.tgz", SHA1: "original-sha"},
						{Name: "diego", Version: "5.6.7", RemoteSource: builtSourceID, RemotePath: "/remote/path/diego-4.5.6.tgz", SHA1: "original-sha"},
						{Name: "route-emitter", Version: "8.9.10", RemoteSource: builtSourceID, RemotePath: "/remote/path/route-emitter-8.9.10.tgz", SHA1: "original-sha"},
						{Name: "bpm", Version: "1.6", RemoteSource: compiledSourceID, RemotePath: "not-used", SHA1: "original-sha"},
					},
					Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
				}
			})

			It("compiles and downloads the compiled releases", func() {
				err := command.Execute([]string{
					"--kilnfile", kilnfilePath,
					"--releases-directory", releasesPath,
					"--stemcell-file", stemcellPath,
					"--upload-target-id", compiledSourceID,
					"--parallel", "3",
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
				Expect(boshDirector.UploadReleaseFileCallCount()).To(Equal(4))

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

				diegoReleaseFile, rebase, fix := boshDirector.UploadReleaseFileArgsForCall(2)
				Expect(fix).To(BeFalse())
				Expect(rebase).To(BeFalse())

				diegoReleaseFileStats, err := diegoReleaseFile.Stat()
				Expect(err).NotTo(HaveOccurred())
				Expect(diegoReleaseFileStats.Name()).To(Equal("diego-5.6.7.tgz"))

				routeEmitterReleaseFile, rebase, fix := boshDirector.UploadReleaseFileArgsForCall(3)
				Expect(fix).To(BeFalse())
				Expect(rebase).To(BeFalse())

				routeEmitterReleaseFileStats, err := routeEmitterReleaseFile.Stat()
				Expect(err).NotTo(HaveOccurred())
				Expect(routeEmitterReleaseFileStats.Name()).To(Equal("route-emitter-8.9.10.tgz"))

				By("Creating a deployments for release compilation")
				Expect(boshDirector.FindDeploymentCallCount()).To(Equal(3))

				deploymentName1 := boshDirector.FindDeploymentArgsForCall(0)
				Expect(deploymentName1).To(MatchRegexp("^compile-built-releases-([[:alnum:]]+)"))

				deploymentName2 := boshDirector.FindDeploymentArgsForCall(1)
				Expect(deploymentName2).To(MatchRegexp("^compile-built-releases-([[:alnum:]]+)"))
				Expect(deploymentName2).NotTo(Equal(deploymentName1))

				deploymentName3 := boshDirector.FindDeploymentArgsForCall(2)
				Expect(deploymentName3).To(MatchRegexp("^compile-built-releases-([[:alnum:]]+)"))
				Expect(deploymentName3).NotTo(Equal(deploymentName2))
				Expect(deploymentName3).NotTo(Equal(deploymentName1))

				Expect(boshDeployment1.UpdateCallCount()).To(Equal(1))
				manifest, options := boshDeployment1.UpdateArgsForCall(0)
				Expect(options).To(BeZero())
				Expect(string(manifest)).To(MatchRegexp(`name: uaa\n\s+version: 1\.2\.3`))
				Expect(string(manifest)).To(MatchRegexp(`name: capi\n\s+version: 2\.3\.4`))
				Expect(string(manifest)).To(MatchRegexp(`name: diego\n\s+version: 5\.6\.7`))
				Expect(string(manifest)).To(MatchRegexp(`name: route-emitter\n\s+version: 8\.9\.10`))
				Expect(string(manifest)).To(MatchRegexp(`alias: default\n\s+os: plan9\n\s+version: "42"`))

				Expect(boshDeployment2.UpdateCallCount()).To(Equal(1))
				manifest, options = boshDeployment2.UpdateArgsForCall(0)
				Expect(options).To(BeZero())
				Expect(string(manifest)).To(MatchRegexp(`name: uaa\n\s+version: 1\.2\.3`))
				Expect(string(manifest)).To(MatchRegexp(`name: capi\n\s+version: 2\.3\.4`))
				Expect(string(manifest)).To(MatchRegexp(`name: diego\n\s+version: 5\.6\.7`))
				Expect(string(manifest)).To(MatchRegexp(`name: route-emitter\n\s+version: 8\.9\.10`))
				Expect(string(manifest)).To(MatchRegexp(`alias: default\n\s+os: plan9\n\s+version: "42"`))

				Expect(boshDeployment3.UpdateCallCount()).To(Equal(1))
				manifest, options = boshDeployment3.UpdateArgsForCall(0)
				Expect(options).To(BeZero())
				Expect(string(manifest)).To(MatchRegexp(`name: uaa\n\s+version: 1\.2\.3`))
				Expect(string(manifest)).To(MatchRegexp(`name: capi\n\s+version: 2\.3\.4`))
				Expect(string(manifest)).To(MatchRegexp(`name: diego\n\s+version: 5\.6\.7`))
				Expect(string(manifest)).To(MatchRegexp(`name: route-emitter\n\s+version: 8\.9\.10`))
				Expect(string(manifest)).To(MatchRegexp(`alias: default\n\s+os: plan9\n\s+version: "42"`))

				By("exporting the compiled releases")
				boshDeployment1CallCount := boshDeployment1.ExportReleaseCallCount()
				boshDeployment2CallCount := boshDeployment2.ExportReleaseCallCount()
				boshDeployment3CallCount := boshDeployment3.ExportReleaseCallCount()
				Expect(boshDeployment1CallCount + boshDeployment2CallCount + boshDeployment3CallCount).To(Equal(4))
				var releasesSlug []string
				for i := 0; i < boshDeployment1CallCount; i++ {
					releaseSlug, osVersionSlug, jobs := boshDeployment1.ExportReleaseArgsForCall(i)
					releasesSlug = append(releasesSlug, releaseSlug.String())
					Expect(osVersionSlug.String()).To(Equal("plan9/42"))
					Expect(jobs).To(BeEmpty())
				}
				for i := 0; i < boshDeployment2CallCount; i++ {
					releaseSlug, osVersionSlug, jobs := boshDeployment2.ExportReleaseArgsForCall(i)
					releasesSlug = append(releasesSlug, releaseSlug.String())
					Expect(osVersionSlug.String()).To(Equal("plan9/42"))
					Expect(jobs).To(BeEmpty())
				}
				for i := 0; i < boshDeployment3CallCount; i++ {
					releaseSlug, osVersionSlug, jobs := boshDeployment3.ExportReleaseArgsForCall(i)
					releasesSlug = append(releasesSlug, releaseSlug.String())
					Expect(osVersionSlug.String()).To(Equal("plan9/42"))
					Expect(jobs).To(BeEmpty())
				}

				Expect(releasesSlug).To(ContainElements("uaa/1.2.3", "capi/2.3.4", "diego/5.6.7", "route-emitter/8.9.10"))

				By("Downloading the underlying exported release from bosh")
				Expect(boshDirector.DownloadResourceUncheckedCallCount()).To(Equal(4))

				var blobstoreIDs []string
				var filehandlerNames []string
				for i := 0; i < 4; i++ {
					blobstoreID, filehandler := boshDirector.DownloadResourceUncheckedArgsForCall(i)
					blobstoreIDs = append(blobstoreIDs, blobstoreID)
					filehandlerNames = append(filehandlerNames, filehandler.(*os.File).Name())
				}
				Expect(blobstoreIDs).To(ContainElements("uaa-1.2.3", "capi-2.3.4", "diego-5.6.7", "route-emitter-8.9.10"))
				Expect(filehandlerNames).To(ContainElements(ContainSubstring("uaa-1.2.3-plan9-42.tgz"), ContainSubstring("capi-2.3.4-plan9-42.tgz"), ContainSubstring("diego-5.6.7-plan9-42.tgz"), ContainSubstring("route-emitter-8.9.10-plan9-42.tgz")))

				By("Cleaning up the deployment")
				Expect(boshDeployment1.DeleteCallCount()).To(Equal(1))
				Expect(boshDeployment1.DeleteArgsForCall(0)).To(BeTrue())
				Expect(boshDeployment2.DeleteCallCount()).To(Equal(1))
				Expect(boshDeployment2.DeleteArgsForCall(0)).To(BeTrue())
				Expect(boshDeployment3.DeleteCallCount()).To(Equal(1))
				Expect(boshDeployment3.DeleteArgsForCall(0)).To(BeTrue())

				Expect(boshDirector.CleanUpCallCount()).To(Equal(1))
				all, dryRun, keepOrphanedDisks := boshDirector.CleanUpArgsForCall(0)
				Expect(all).To(BeTrue())
				Expect(dryRun).To(BeFalse())
				Expect(keepOrphanedDisks).To(BeFalse())
			})
		})
	})

	When("all of the releases are already compiled in the Kilnfile.lock", func() {
		BeforeEach(func() {
			kilnfileLock = cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{Name: "uaa", Version: "1.2.3", RemoteSource: compiledSourceID, RemotePath: "not-used", SHA1: "original-sha"},
					{Name: "capi", Version: "2.3.4", RemoteSource: compiledSourceID, RemotePath: "not-used", SHA1: "original-sha"},
					{Name: "bpm", Version: "1.6", RemoteSource: compiledSourceID, RemotePath: "not-used", SHA1: "original-sha"},
				},
				Stemcell: cargo.Stemcell{OS: stemcellOS, Version: stemcellVersion},
			}
		})
		It("doesn't compile any releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(boshDirector.FindDeploymentCallCount()).To(Equal(0))
			Expect(boshDirector.UploadReleaseFileCallCount()).To(Equal(0))
		})

		It("doesn't upload any releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
		})

		It("doesn't download anything", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(compiledReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
			Expect(builtReleaseSource.DownloadReleaseCallCount()).To(Equal(0))
		})

		It("doesn't update the Kilnfile.lock", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
		})
	})

	When("one of the releases have already been compiled and uploaded", func() {
		const (
			expectedUAASHA        = "updated-uaa-sha"
			expectedUAARemotePath = "compiled-uaa-remote-path"
		)

		BeforeEach(func() {
			uaaID := release.ID{Name: "uaa", Version: "1.2.3"}

			compiledReleaseSource.GetMatchedReleaseCalls(func(requirement release.Requirement) (release.Remote, bool, error) {
				if requirement.Name == "uaa" {
					return release.Remote{
						ID:         uaaID,
						RemotePath: expectedUAARemotePath,
						SourceID:   compiledSourceID,
					}, true, nil
				}
				return release.Remote{}, false, nil
			})
			compiledReleaseSource.DownloadReleaseReturns(release.Local{
				ID:        uaaID,
				LocalPath: "not-used",
				SHA1:      expectedUAASHA,
			}, nil)
		})

		It("doesn't compile that release", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(boshDirector.UploadReleaseFileCallCount()).To(Equal(1))

			capiReleaseFile, rebase, fix := boshDirector.UploadReleaseFileArgsForCall(0)
			Expect(fix).To(BeFalse())
			Expect(rebase).To(BeFalse())

			capiReleaseFileStats, err := capiReleaseFile.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(capiReleaseFileStats.Name()).To(Equal("capi-2.3.4.tgz"))
		})

		It("doesn't upload that release", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(1))

			spec, releaseFile := releaseUploader.UploadReleaseArgsForCall(0)
			Expect(spec).To(Equal(release.Requirement{
				Name:            "capi",
				Version:         "2.3.4",
				StemcellOS:      "plan9",
				StemcellVersion: "42",
			}))
			contents, err := ioutil.ReadAll(releaseFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("contents of capi-2.3.4"))
		})

		It("downloads the pre-compiled release", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(compiledReleaseSource.DownloadReleaseCallCount()).To(Equal(1))

			downloadDir, remoteRelease, _ := compiledReleaseSource.DownloadReleaseArgsForCall(0)
			Expect(downloadDir).To(Equal(releasesPath))
			Expect(remoteRelease).To(Equal(release.Remote{
				ID:         release.ID{Name: "uaa", Version: "1.2.3"},
				RemotePath: expectedUAARemotePath,
				SourceID:   compiledSourceID,
			}))
		})

		It("updates the Kilnfile.lock with that release", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			s := sha1.New()
			io.Copy(s, strings.NewReader(blobIDContents("capi-2.3.4")))
			expectedCapiSha := hex.EncodeToString(s.Sum(nil))

			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

			_, path, updatedLockfile := kilnfileLoader.SaveKilnfileLockArgsForCall(0)
			Expect(path).To(Equal(kilnfilePath))
			Expect(updatedLockfile).To(Equal(cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         "uaa",
						Version:      "1.2.3",
						RemoteSource: compiledSourceID,
						RemotePath:   expectedUAARemotePath,
						SHA1:         expectedUAASHA,
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

	When("all of the releases have already been compiled and uploaded", func() {
		const (
			expectedUAASHA         = "updated-uaa-sha"
			expectedUAARemotePath  = "compiled-uaa-remote-path"
			expectedCAPISHA        = "updated-capi-sha"
			expectedCAPIRemotePath = "compiled-capi-remote-path"
		)

		BeforeEach(func() {
			uaaID := release.ID{Name: "uaa", Version: "1.2.3"}
			capiID := release.ID{Name: "capi", Version: "2.3.4"}

			compiledReleaseSource.GetMatchedReleaseCalls(func(requirement release.Requirement) (release.Remote, bool, error) {
				switch requirement.Name {
				case "uaa":
					return release.Remote{
						ID:         uaaID,
						RemotePath: expectedUAARemotePath,
						SourceID:   compiledSourceID,
					}, true, nil
				case "capi":
					return release.Remote{
						ID:         capiID,
						RemotePath: expectedCAPIRemotePath,
						SourceID:   compiledSourceID,
					}, true, nil
				default:
					return release.Remote{}, false, nil
				}
			})

			compiledReleaseSource.DownloadReleaseCalls(func(_ string, remote release.Remote, _ int) (release.Local, error) {
				switch remote.Name {
				case "uaa":
					return release.Local{
						ID:        uaaID,
						LocalPath: "not-used",
						SHA1:      expectedUAASHA,
					}, nil
				case "capi":
					return release.Local{
						ID:        capiID,
						LocalPath: "not-used",
						SHA1:      expectedCAPISHA,
					}, nil
				default:
					return release.Local{}, nil
				}
			})
		})

		It("doesn't compile any releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(boshDirector.FindDeploymentCallCount()).To(Equal(0))
			Expect(boshDirector.UploadReleaseFileCallCount()).To(Equal(0))
		})

		It("doesn't upload any releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(releaseUploader.UploadReleaseCallCount()).To(Equal(0))
		})

		It("downloads the pre-compiled releases", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(compiledReleaseSource.DownloadReleaseCallCount()).To(Equal(2))

			downloadDir, remoteRelease, _ := compiledReleaseSource.DownloadReleaseArgsForCall(0)
			Expect(downloadDir).To(Equal(releasesPath))
			Expect(remoteRelease).To(Equal(release.Remote{
				ID:         release.ID{Name: "uaa", Version: "1.2.3"},
				RemotePath: expectedUAARemotePath,
				SourceID:   compiledSourceID,
			}))

			downloadDir, remoteRelease, _ = compiledReleaseSource.DownloadReleaseArgsForCall(1)
			Expect(downloadDir).To(Equal(releasesPath))
			Expect(remoteRelease).To(Equal(release.Remote{
				ID:         release.ID{Name: "capi", Version: "2.3.4"},
				RemotePath: expectedCAPIRemotePath,
				SourceID:   compiledSourceID,
			}))
		})

		It("updates the Kilnfile.lock with those releases", func() {
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
			Expect(updatedLockfile).To(Equal(cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         "uaa",
						Version:      "1.2.3",
						RemoteSource: compiledSourceID,
						RemotePath:   expectedUAARemotePath,
						SHA1:         expectedUAASHA,
					},
					{
						Name:         "capi",
						Version:      "2.3.4",
						RemoteSource: compiledSourceID,
						RemotePath:   expectedCAPIRemotePath,
						SHA1:         expectedCAPISHA,
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
			boshDeployment1.ExportReleaseReturns(boshdir.ExportReleaseResult{}, errors.New(errorMsg))
		})

		It("still deletes the compilation deployment and cleans up director assets", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
			Expect(err).To(MatchError(MatchRegexp("capi|uaa")))

			Expect(boshDeployment1.DeleteCallCount()).To(Equal(1))
			force := boshDeployment1.DeleteArgsForCall(0)
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
			Expect(err).To(MatchError(MatchRegexp("uaa|capi")))

			Expect(boshDeployment1.DeleteCallCount()).To(Equal(1))
			force := boshDeployment1.DeleteArgsForCall(0)
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
			boshDeployment1.DeleteReturns(errors.New("panic now >:DDD"))
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
			Expect(err).To(MatchError(ContainSubstring("couldn't find")))
			Expect(err).To(MatchError(ContainSubstring("no-such-source")))
		})
	})

	When("a downloaded compiled release has an incorrect SHA", func() {
		BeforeEach(func() {
			boshDeployment1.ExportReleaseReturns(boshdir.ExportReleaseResult{
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
			Expect(err).To(MatchError(MatchRegexp("uaa|capi")))
		})
	})

	When("searching for pre-compiled releases fails", func() {
		BeforeEach(func() {
			compiledReleaseSource.GetMatchedReleaseReturns(release.Remote{}, false, errors.New("boom today"))
		})

		It("errors", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring("uaa")))
			Expect(err).To(MatchError(ContainSubstring("boom today")))
		})

		It("doesn't update the Kilnfile.lock", func() {
			_ = command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
		})
	})

	When("downloading a pre-compiled releases fails", func() {
		BeforeEach(func() {
			compiledReleaseSource.GetMatchedReleaseReturns(release.Remote{SourceID: compiledSourceID}, true, nil)
			compiledReleaseSource.DownloadReleaseReturns(release.Local{}, errors.New("NOPE!"))
		})

		It("errors", func() {
			err := command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(err).To(MatchError(ContainSubstring("uaa")))
			Expect(err).To(MatchError(ContainSubstring("NOPE!")))
		})

		It("doesn't update the Kilnfile.lock", func() {
			_ = command.Execute([]string{
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesPath,
				"--stemcell-file", stemcellPath,
				"--upload-target-id", compiledSourceID,
			})
			Expect(kilnfileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
		})
	})
})
