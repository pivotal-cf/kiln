package commands

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsystem "github.com/cloudfoundry/bosh-utils/system"
	"github.com/google/uuid"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/helper"
	"github.com/pivotal-cf/kiln/internal/manifest_generator"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type CompileBuiltReleases struct {
	Logger                     *log.Logger
	MultiReleaseSourceProvider MultiReleaseSourceProvider
	ReleaseUploaderFinder      ReleaseUploaderFinder
	BoshDirectorFactory        func() (BoshDirector, error)

	Options struct {
		flags.Standard

		ReleasesDir    string `short:"rd" long:"releases-directory" default:"releases" description:"path to a directory to download releases into"`
		StemcellFile   string `short:"sf" long:"stemcell-file"      required:"true"    description:"path to the stemcell tarball on disk"`
		UploadTargetID string `           long:"upload-target-id"   required:"true"    description:"the ID of the release source where the compiled release will be uploaded"`
		Parallel       int64  `short:"p" long:"parallel" default:"1" description:"number of parallel compile release jobs"`
	}
}

//counterfeiter:generate -o ./fakes/bosh_deployment.go --fake-name BoshDeployment github.com/cloudfoundry/bosh-cli/director.Deployment

//counterfeiter:generate -o ./fakes/bosh_director.go --fake-name BoshDirector . BoshDirector
type BoshDirector interface {
	UploadStemcellFile(file boshdir.UploadFile, fix bool) error
	UploadReleaseFile(file boshdir.UploadFile, rebase, fix bool) error
	FindDeployment(name string) (boshdir.Deployment, error)
	DownloadResourceUnchecked(blobstoreID string, out io.Writer) error
	CleanUp(all bool, dryRun bool, keepOrphanedDisks bool) (boshdir.CleanUp, error)
}

func BoshDirectorFactory() (BoshDirector, error) {
	boshURL := os.Getenv("BOSH_ENVIRONMENT")
	boshClient := os.Getenv("BOSH_CLIENT")
	boshClientSecret := os.Getenv("BOSH_CLIENT_SECRET")
	boshCA := os.Getenv("BOSH_CA_CERT")

	logger := boshlog.NewLogger(boshlog.LevelError)
	factory := boshdir.NewFactory(logger)

	config, err := boshdir.NewConfigFromURL(boshURL)
	if err != nil {
		return nil, err
	}

	config.CACert = boshCA

	basicDirector, err := factory.New(config, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	if err != nil {
		return nil, err
	}

	info, err := basicDirector.Info()
	if err != nil {
		return nil, fmt.Errorf("could not get basic director info: %s", err)
	}

	uaaClientFactory := boshuaa.NewFactory(logger)

	uaaConfig, err := boshuaa.NewConfigFromURL(info.Auth.Options["url"].(string))
	if err != nil {
		return nil, err
	}

	uaaConfig.Client = boshClient
	uaaConfig.ClientSecret = boshClientSecret
	uaaConfig.CACert = boshCA

	uaa, err := uaaClientFactory.New(uaaConfig)
	if err != nil {
		return nil, fmt.Errorf("could not build uaa auth from director info: %s", err)
	}

	config.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc

	return factory.New(config, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
}

func (f CompileBuiltReleases) Execute(args []string) error {
	err := flags.LoadFlagsWithDefaults(&f.Options, args, nil)
	if err != nil {
		return err
	}

	f.Logger.Println("loading Kilnfile")
	kilnfile, kilnfileLock, err := f.Options.LoadKilnfiles(nil, nil)
	if err != nil {
		return fmt.Errorf("couldn't load Kilnfiles: %w", err) // untested
	}

	publishableReleaseSources := f.MultiReleaseSourceProvider(kilnfile, true)
	allReleaseSources := f.MultiReleaseSourceProvider(kilnfile, false)
	releaseUploader, err := f.ReleaseUploaderFinder(kilnfile, f.Options.UploadTargetID)
	if err != nil {
		return fmt.Errorf("error loading release uploader: %w", err) // untested
	}

	builtReleases, err := findBuiltReleases(allReleaseSources, kilnfileLock)
	if err != nil {
		return err
	}

	if len(builtReleases) == 0 {
		f.Logger.Println("All releases are compiled. Exiting early")
		return nil
	}

	updatedReleases, remainingBuiltReleases, err := f.downloadPreCompiledReleases(publishableReleaseSources, builtReleases, kilnfileLock.Stemcell)
	if err != nil {
		return err
	}

	if len(remainingBuiltReleases) > 0 {
		f.Logger.Printf("need to compile %d built releases\n", len(remainingBuiltReleases))

		downloadedReleases, stemcell, err := f.compileAndDownloadReleases(allReleaseSources, remainingBuiltReleases)
		if err != nil {
			return err
		}

		uploadedReleases, err := f.uploadCompiledReleases(downloadedReleases, releaseUploader, stemcell)
		if err != nil {
			return err
		}
		updatedReleases = append(updatedReleases, uploadedReleases...)
	} else {
		f.Logger.Println("nothing left to compile")
	}

	err = f.updateLockfile(updatedReleases, kilnfileLock)
	if err != nil {
		return err
	}

	f.Logger.Println("Updated Kilnfile.lock. DONE")
	return nil
}

func (f CompileBuiltReleases) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Compiles built releases in the Kilnfile.lock and uploads them to the release source",
		ShortDescription: "compiles built releases and uploads them",
		Flags:            f.Options,
	}
}

type remoteReleaseWithSHA1 struct {
	component.Lock
	SHA1 string
}

func findBuiltReleases(allReleaseSources component.MultiReleaseSource, kilnfileLock cargo.KilnfileLock) ([]component.Lock, error) {
	var builtReleases []component.Lock
	for _, lock := range kilnfileLock.Releases {
		src, err := allReleaseSources.FindByID(lock.RemoteSource)
		if err != nil {
			return nil, err
		}
		if !src.Configuration().Publishable {
			releaseID := component.Spec{Name: lock.Name, Version: lock.Version}
			builtReleases = append(builtReleases, releaseID.Lock().WithRemote(lock.RemoteSource, lock.RemotePath))
		}
	}
	return builtReleases, nil
}

func (f CompileBuiltReleases) downloadPreCompiledReleases(publishableReleaseSources component.MultiReleaseSource, builtReleases []component.Lock, stemcell cargo.Stemcell) ([]remoteReleaseWithSHA1, []component.Lock, error) {
	var (
		remainingBuiltReleases []component.Lock
		preCompiledReleases    []remoteReleaseWithSHA1
	)

	f.Logger.Println("searching for pre-compiled releases")

	for _, builtRelease := range builtReleases {
		spec := component.Spec{
			Name:            builtRelease.Name,
			Version:         builtRelease.Version,
			StemcellOS:      stemcell.OS,
			StemcellVersion: stemcell.Version,
		}
		remote, found, err := publishableReleaseSources.GetMatchedRelease(spec)
		if err != nil {
			return nil, nil, fmt.Errorf("error searching for pre-compiled release for %q: %w", builtRelease.Name, err)
		}
		if !found {
			remainingBuiltReleases = append(remainingBuiltReleases, builtRelease)
			continue
		}

		err = publishableReleaseSources.DownloadComponent(f.Options.ReleasesDir, remote)
		if err != nil {
			return nil, nil, fmt.Errorf("error downloading pre-compiled release for %q: %w", builtRelease.Name, err)
		}

		preCompiledReleases = append(preCompiledReleases, remoteReleaseWithSHA1{Lock: remote, SHA1: local.SHA1})
	}

	f.Logger.Printf("found %d pre-compiled releases\n", len(preCompiledReleases))

	return preCompiledReleases, remainingBuiltReleases, nil
}

func (f CompileBuiltReleases) compileAndDownloadReleases(releaseSource component.MultiReleaseSource, builtReleases []component.Lock) ([]component.Local, builder.StemcellManifest, error) {
	f.Logger.Println("connecting to the bosh director")
	boshDirector, err := f.BoshDirectorFactory()
	if err != nil {
		return nil, builder.StemcellManifest{}, fmt.Errorf("unable to connect to bosh director: %w", err) // untested
	}

	releaseIDs, err := f.uploadReleasesToDirector(builtReleases, releaseSource, boshDirector)
	if err != nil {
		return nil, builder.StemcellManifest{}, err
	}

	stemcellManifest, err := f.uploadStemcellToDirector(boshDirector)
	if err != nil {
		return nil, builder.StemcellManifest{}, err
	}

	var deployments []boshdir.Deployment
	for i := 0; i < int(f.Options.Parallel); i++ {
		deploymentName := fmt.Sprintf("compile-built-releases-%d-%s", i, uuid.Must(uuid.NewRandom()))
		f.Logger.Printf("deploying compilation deployment %q\n", deploymentName)
		deployment, err := boshDirector.FindDeployment(deploymentName)
		if err != nil {
			return nil, builder.StemcellManifest{}, fmt.Errorf("couldn't create deployment: %w", err) // untested
		}
		deployments = append(deployments, deployment)

		mg := manifest_generator.New()
		manifest, err := mg.Generate(deploymentName, releaseIDs, stemcellManifest)
		if err != nil {
			return nil, builder.StemcellManifest{}, fmt.Errorf("couldn't generate bosh manifest: %v", err) // untested
		}

		err = deployment.Update(manifest, boshdir.UpdateOpts{})
		if err != nil {
			return nil, builder.StemcellManifest{}, fmt.Errorf("updating the bosh deployment: %v", err) // untested
		}
	}

	defer func() {
		f.Logger.Println("deleting compilation deployments")
		for _, deployment := range deployments {
			err = deployment.Delete(true)
			if err != nil {
				panic(fmt.Errorf("error deleting the deployment: %w", err))
			}
		}

		f.Logger.Println("cleaning up unused releases and stemcells")
		_, err = boshDirector.CleanUp(true, false, false)
		if err != nil {
			f.Logger.Println(fmt.Sprintf("warning: bosh director failed cleanup with the following error: %v", err))
			return
		}
	}()

	downloadedReleases, err := f.downloadCompiledReleases(stemcellManifest, releaseIDs, deployments, boshDirector)
	if err != nil {
		return nil, builder.StemcellManifest{}, err // untested
	}

	return downloadedReleases, stemcellManifest, nil
}

func (f CompileBuiltReleases) uploadReleasesToDirector(builtReleases []component.Lock, releaseSource component.MultiReleaseSource, boshDirector BoshDirector) ([]component.Spec, error) {
	var releaseIDs []component.Spec
	for _, remoteRelease := range builtReleases {
		releaseIDs = append(releaseIDs, remoteRelease.Spec())

		localRelease, err := releaseSource.DownloadRelease(f.Options.ReleasesDir, remoteRelease)
		if err != nil {
			return nil, fmt.Errorf("failure downloading built release %s@%s: %w", remoteRelease.Name, remoteRelease.Version, err) // untested
		}

		builtReleaseForUploading, err := os.Open(localRelease.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("opening local built release %q: %w", localRelease.LocalPath, err) // untested
		}

		f.Logger.Printf("uploading release %q to director\n", localRelease.LocalPath)
		err = boshDirector.UploadReleaseFile(builtReleaseForUploading, false, false)
		if err != nil {
			return nil, fmt.Errorf("failure uploading release %q to bosh director: %w", localRelease.LocalPath, err) // untested
		}
	}
	return releaseIDs, nil
}

func (f CompileBuiltReleases) uploadStemcellToDirector(boshDirector BoshDirector) (builder.StemcellManifest, error) {
	f.Logger.Printf("uploading stemcell %q to director\n", f.Options.StemcellFile)
	stemcellFile, err := os.Open(f.Options.StemcellFile)
	if err != nil {
		return builder.StemcellManifest{}, fmt.Errorf("opening stemcell: %w", err) // untested
	}

	err = boshDirector.UploadStemcellFile(stemcellFile, false)
	if err != nil {
		return builder.StemcellManifest{}, fmt.Errorf("failure uploading stemcell to bosh director: %w", err) // untested
	}

	stemcellManifestReader := builder.NewStemcellManifestReader(helper.NewFilesystem())
	stemcellPart, err := stemcellManifestReader.Read(f.Options.StemcellFile)
	if err != nil {
		return builder.StemcellManifest{}, fmt.Errorf("couldn't parse manifest of stemcell: %v", err) // untested
	}

	stemcellManifest := stemcellPart.Metadata.(builder.StemcellManifest)
	return stemcellManifest, err
}

func (f CompileBuiltReleases) downloadCompiledReleases(stemcellManifest builder.StemcellManifest, releaseIDs []component.Spec, deployments []boshdir.Deployment, boshDirector BoshDirector) ([]component.Local, error) {
	var downloadedReleases []component.Local
	exportedReleases, err := f.exportReleasesInParallel(stemcellManifest, deployments, releaseIDs)
	if err != nil {
		return nil, err
	}

	for _, rel := range exportedReleases {
		fd, err := os.OpenFile(rel.TarballPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("creating compiled release file %s: %w", rel.TarballPath, err) // untested
		}

		f.Logger.Printf("downloading release %q from director\n", rel.Name)
		err = boshDirector.DownloadResourceUnchecked(rel.BlobstoreID, fd)
		if err != nil {
			return nil, fmt.Errorf("downloading exported release %s: %w", rel.Name, err)
		}

		err = fd.Close()
		if err != nil {
			return nil, fmt.Errorf("failed closing file %s: %w", rel.TarballPath, err) // untested
		}

		fd, err = os.Open(rel.TarballPath)
		if err != nil {
			return nil, fmt.Errorf("failed reopening file %s: %w", rel.TarballPath, err) // untested
		}

		s := sha1.New()
		_, err = io.Copy(s, fd)
		if err != nil {
			return nil, fmt.Errorf("failed calculating SHA1 for file file %s: %w", rel.TarballPath, err) // untested
		}
		err = fd.Close()
		if err != nil {
			return nil, fmt.Errorf("failed closing file %s: %w", rel.TarballPath, err) // untested
		}

		downloadedReleases = append(downloadedReleases, component.Local{
			Lock:      rel.Lock.WithSHA1(hex.EncodeToString(s.Sum(nil))),
			LocalPath: rel.TarballPath,
		})

		expectedMultipleDigest, err := boshcrypto.ParseMultipleDigest(rel.SHA1)
		if err != nil {
			return nil, fmt.Errorf("error parsing SHA of downloaded release %q: %w", rel.Name, err) // untested
		}

		ignoreMeLogger := boshlog.NewLogger(boshlog.LevelNone)
		fs := boshsystem.NewOsFileSystem(ignoreMeLogger)
		err = expectedMultipleDigest.VerifyFilePath(rel.TarballPath, fs)
		if err != nil {
			return nil, fmt.Errorf("compiled release %q has an incorrect SHA: %w", rel.Name, err)
		}
	}

	return downloadedReleases, nil
}

func (f CompileBuiltReleases) exportReleasesInParallel(stemcellManifest builder.StemcellManifest, deployments []boshdir.Deployment, releaseIDs []component.Spec) ([]component.Exported, error) {
	osVersionSlug := boshdir.NewOSVersionSlug(stemcellManifest.OperatingSystem, stemcellManifest.Version)

	errCh := make(chan error, len(releaseIDs))
	wg := sync.WaitGroup{}
	var exportedReleases []component.Exported
	exportedReleasesMux := sync.Mutex{}

	deploymentsPool := make(chan boshdir.Deployment, len(deployments))
	for _, deployment := range deployments {
		deploymentsPool <- deployment
	}
	cancelCh := make(chan error, 1)

	for _, releaseID := range releaseIDs {
		wg.Add(1)

		go func(rel component.Spec) {
			defer wg.Done()

			var deployment boshdir.Deployment

			select {
			case <-cancelCh:
				return
			case deployment = <-deploymentsPool:
				defer func() {
					// releasing deployment
					deploymentsPool <- deployment
				}()

				compiledTarballPath := filepath.Join(f.Options.ReleasesDir, fmt.Sprintf("%s-%s-%s-%s.tgz", rel.Name, rel.Version, stemcellManifest.OperatingSystem, stemcellManifest.Version))
				f.Logger.Printf("exporting release %q\n", compiledTarballPath)

				result, err := deployment.ExportRelease(boshdir.NewReleaseSlug(rel.Name, rel.Version), osVersionSlug, nil)
				if err != nil {
					errCh <- fmt.Errorf("exporting release %s: %w", rel.Name, err)
					return
				}

				exportedReleasesMux.Lock()
				exportedReleases = append(exportedReleases, component.Exported{
					Lock: component.Lock{
						Name:    rel.Name,
						Version: rel.Version,
						SHA1:    result.SHA1,
					},
					TarballPath: compiledTarballPath,
					BlobstoreID: result.BlobstoreID,
				})
				exportedReleasesMux.Unlock()

				errCh <- nil
				return
			}
		}(releaseID)
	}

	var err error
	for i := 0; i < len(releaseIDs); i++ {
		e := <-errCh
		if e != nil {
			close(cancelCh)
			err = e
			break
		}
	}

	wg.Wait()

	if err != nil {
		return nil, err
	}

	exportedReleasesMux.Lock()
	defer exportedReleasesMux.Unlock()
	return exportedReleases, nil
}

func (f CompileBuiltReleases) uploadCompiledReleases(downloadedReleases []component.Local, releaseUploader component.ReleaseUploader, stemcell builder.StemcellManifest) ([]remoteReleaseWithSHA1, error) {
	var uploadedReleases []remoteReleaseWithSHA1

	for _, downloadedRelease := range downloadedReleases {
		releaseFile, err := os.Open(downloadedRelease.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("opening compiled release %q for uploading: %w", downloadedRelease.LocalPath, err) // untested
		}

		remoteRelease, err := releaseUploader.UploadRelease(component.Spec{
			Name:            downloadedRelease.Name,
			StemcellOS:      stemcell.OperatingSystem,
			StemcellVersion: stemcell.Version,
			Version:         downloadedRelease.Version,
		}, releaseFile)
		if err != nil {
			return nil, fmt.Errorf("uploading compiled release %q failed: %w", downloadedRelease.LocalPath, err) // untested
		}

		uploadedReleases = append(uploadedReleases, remoteReleaseWithSHA1{Lock: remoteRelease, SHA1: downloadedRelease.SHA1})
	}
	return uploadedReleases, nil
}

func (f CompileBuiltReleases) updateLockfile(uploadedReleases []remoteReleaseWithSHA1, kilnfileLock cargo.KilnfileLock) error {
	for _, uploaded := range uploadedReleases {
		var matchingRelease *cargo.ComponentLock
		for i := range kilnfileLock.Releases {
			if kilnfileLock.Releases[i].Name == uploaded.Name {
				matchingRelease = &kilnfileLock.Releases[i]
				break
			}
		}
		if matchingRelease == nil {
			return fmt.Errorf("no release named %q exists in your Kilnfile.lock", uploaded.Name) // untested (shouldn't be possible)
		}

		matchingRelease.RemoteSource = uploaded.RemoteSource
		matchingRelease.RemotePath = uploaded.RemotePath
		matchingRelease.SHA1 = uploaded.SHA1
	}

	return f.Options.SaveKilnfileLock(nil, kilnfileLock)
}
