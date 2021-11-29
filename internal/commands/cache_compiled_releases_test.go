package commands_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/cloudfoundry/bosh-cli/director"
	boshdirFakes "github.com/cloudfoundry/bosh-cli/director/directorfakes"
	"github.com/go-git/go-billy/v5/memfs"
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	componentFakes "github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/internal/om"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ jhanda.Command = (*commands.CacheCompiledReleases)(nil)

func TestNewCacheCompiledReleases(t *testing.T) {
	please := Ω.NewWithT(t)
	cmd := commands.NewCacheCompiledReleases()
	please.Expect(cmd).NotTo(Ω.BeNil())
	please.Expect(cmd.Logger).NotTo(Ω.BeNil())
	please.Expect(cmd.FS).NotTo(Ω.BeNil())
	please.Expect(cmd.ReleaseCache).NotTo(Ω.BeNil())
	please.Expect(cmd.Bucket).NotTo(Ω.BeNil())
	please.Expect(cmd.OpsManager).NotTo(Ω.BeNil())
	please.Expect(cmd.Director).NotTo(Ω.BeNil())
}

func TestCacheCompiledReleases_Execute_all_releases_are_already_compiled(t *testing.T) {
	please := Ω.NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				ID: "cached-compiled-releases",
			},
		},
	})).NotTo(Ω.HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ComponentLock{
			{
				Name:         "banana",
				Version:      "2.0.0",
				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	})).NotTo(Ω.HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.0"}]}`, nil)

	cache := new(componentFakes.MultiReleaseSource)
	cache.GetMatchedReleaseCalls(fakeCacheData)

	deployment := new(boshdirFakes.FakeDeployment)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.FindDeploymentReturns(deployment, nil)

	bucket := new(fakes.ReleaseCacheBucket)

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheCompiledReleases{
		FS:     fs,
		Logger: logger,
		Bucket: func(kilnfile cargo.Kilnfile) (commands.ReleaseCacheBucket, error) {
			return bucket, nil
		},
		ReleaseCache: func(kilnfile cargo.Kilnfile, targetID string) component.MultiReleaseSource {
			return cache
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "compiled-releases",
	})

	// check

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(output.String()).To(Ω.ContainSubstring("cache already contains releases"))
}

// this test covers
// - an export, download, upload, and lock of a non-cached release
// - an update the kilnfile with a non-locked release cached in the database
//   (the release is cached on s3 but not set in the lock file)
// - ignoring a locked and cached release
//   (the release is cached on the s3 bucket and the lock already has that value in it)
func TestCacheCompiledReleases_Execute_when_one_release_is_cached_another_is_already_compiled_and_another_is_already_locked(t *testing.T) {
	please := Ω.NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				ID:           "cached-compiled-releases",
				Publishable:  true,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
			{
				ID:           "new-releases",
				Publishable:  false,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
		},
	})).NotTo(Ω.HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ComponentLock{
			{

				Name:    "orange",
				Version: "1.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "orange-1.0.0",
			},
			{

				Name:    "banana",
				Version: "2.0.0",

				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",
			},
			{

				Name:    "lemon",
				Version: "3.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "lemon-3.0.0",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	})).NotTo(Ω.HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.0"}]}`, nil)

	cache := new(componentFakes.MultiReleaseSource)
	cache.GetMatchedReleaseCalls(fakeCacheData)

	releaseInBlobstore := []byte(`lemon-release-buffer`)

	deployment := new(boshdirFakes.FakeDeployment)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.FindDeploymentReturns(deployment, nil)
	deployment.ExportReleaseReturns(director.ExportReleaseResult{
		BlobstoreID: "some-blob-id",
		SHA1:        fmt.Sprintf("sha256:%x", sha256.Sum256(releaseInBlobstore)),
	}, nil)
	bosh.DownloadResourceUncheckedCalls(func(_ string, writer io.Writer) error {
		_, _ = writer.Write(releaseInBlobstore)
		return nil
	})

	bucket := new(fakes.ReleaseCacheBucket)
	var uploadedRelease bytes.Buffer
	bucket.UploadReleaseCalls(func(_ component.Spec, reader io.Reader) (component.Lock, error) {
		_, _ = io.Copy(&uploadedRelease, reader)
		return component.Lock{

			Name: "lemon", Version: "3.0.0",

			RemoteSource: "cached-compiled-releases",
			RemotePath:   "lemon-3.0.0-alpine-9.0.0",
			SHA1:         "012ed191f1d07c14bbcbbc0423d0de1c56757348",
		}, nil
	})

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheCompiledReleases{
		FS:     fs,
		Logger: logger,
		Bucket: func(kilnfile cargo.Kilnfile) (commands.ReleaseCacheBucket, error) {
			return bucket, nil
		},
		ReleaseCache: func(kilnfile cargo.Kilnfile, targetID string) component.MultiReleaseSource {
			return cache
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(cache.GetMatchedReleaseCallCount()).To(Ω.Equal(3))
	please.Expect(bosh.DownloadResourceUncheckedCallCount()).To(Ω.Equal(1))

	requestedID, _ := bosh.DownloadResourceUncheckedArgsForCall(0)
	please.Expect(requestedID).To(Ω.Equal("some-blob-id"))

	please.Expect(output.String()).To(Ω.ContainSubstring("not publishable"))
	please.Expect(output.String()).To(Ω.ContainSubstring("lemon 3.0.0 compiled with alpine 9.0.0 not found in cache"))
	please.Expect(output.String()).To(Ω.ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(output.String()).To(Ω.ContainSubstring("exporting lemon"))
	please.Expect(output.String()).To(Ω.ContainSubstring("downloading lemon"))
	please.Expect(output.String()).To(Ω.ContainSubstring("uploading lemon"))
	please.Expect(output.String()).To(Ω.ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))

	please.Expect(uploadedRelease.String()).To(Ω.Equal(string(releaseInBlobstore)))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedKilnfile)).NotTo(Ω.HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(Ω.ContainElement(component.Lock{
		Name:         "lemon",
		Version:      "3.0.0",
		SHA1:         "012ed191f1d07c14bbcbbc0423d0de1c56757348",
		RemoteSource: "cached-compiled-releases",
		RemotePath:   "lemon-3.0.0-alpine-9.0.0",
	}))
}

// this test ensures make it so the we don't have to iterate over all the releases
// before failing due to a stemcell mismatch
func TestCacheCompiledReleases_Execute_staged_and_lock_stemcells_are_not_the_same(t *testing.T) {
	please := Ω.NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				ID:           "cached-compiled-releases",
				Publishable:  true,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
			{
				ID:           "new-releases",
				Publishable:  false,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
		},
	})).NotTo(Ω.HaveOccurred())
	initialLock := cargo.KilnfileLock{
		Releases: []component.Lock{
			{

				Name:    "orange",
				Version: "1.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "orange-1.0.0",
			},
			{

				Name:    "banana",
				Version: "2.0.0",

				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",
			},
			{

				Name:    "lemon",
				Version: "3.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "lemon-3.0.0",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	}
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", initialLock)).NotTo(Ω.HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.1"}]}`, nil)

	cache := new(componentFakes.MultiReleaseSource)
	cache.GetMatchedReleaseCalls(fakeCacheData)

	bosh := new(boshdirFakes.FakeDirector)
	bucket := new(fakes.ReleaseCacheBucket)

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheCompiledReleases{
		FS:     fs,
		Logger: logger,
		Bucket: func(kilnfile cargo.Kilnfile) (commands.ReleaseCacheBucket, error) {
			return bucket, nil
		},
		ReleaseCache: func(kilnfile cargo.Kilnfile, targetID string) component.MultiReleaseSource {
			return cache
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(cache.GetMatchedReleaseCallCount()).To(Ω.Equal(0))
	please.Expect(bosh.DownloadResourceUncheckedCallCount()).To(Ω.Equal(0))
	please.Expect(err).To(Ω.MatchError(Ω.Equal("staged stemcell (alpine 9.0.1) and lock stemcell (alpine 9.0.0) do not match")))

	var updatedLock cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedLock)).NotTo(Ω.HaveOccurred())
	please.Expect(updatedLock).To(Ω.Equal(initialLock))
}

func fakeCacheData(spec component.Spec) (component.Lock, bool, error) {
	switch spec.Lock() {
	case component.Lock{Name: "orange", Version: "1.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
		return component.Lock{

			Name: "orange", Version: "1.0.0",

			RemoteSource: "cached-compiled-releases",
			RemotePath:   "orange-1.0.0-alpine-9.0.0",
		}, true, nil
	case component.Lock{Name: "banana", Version: "2.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
		return component.Lock{

			Name: "banana", Version: "2.0.0",

			RemoteSource: "cached-compiled-releases",
			RemotePath:   "banana-2.0.0-alpine-9.0.0",
		}, true, nil
	case component.Lock{Name: "lemon", Version: "3.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
		return component.Lock{}, false, nil
	}

	panic(fmt.Sprintf("unexpected spec %#v", spec))
}
