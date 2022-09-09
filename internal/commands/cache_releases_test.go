package commands_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/om"
	"github.com/pivotal-cf/kiln/pkg/cargo"

	boshdirFakes "github.com/cloudfoundry/bosh-cli/director/directorfakes"
	. "github.com/onsi/gomega"
	component_fakes "github.com/pivotal-cf/kiln/internal/component/fakes"
)

var _ jhanda.Command = (*commands.CacheReleases)(nil)

func TestNewCacheCompiledReleases(t *testing.T) {
	please := NewWithT(t)
	cmd := commands.NewCacheReleases()
	please.Expect(cmd).NotTo(BeNil())
	please.Expect(cmd.Logger).NotTo(BeNil())
	please.Expect(cmd.FS).NotTo(BeNil())
	please.Expect(cmd.ReleaseSourceAndCache).NotTo(BeNil())
	please.Expect(cmd.OpsManager).NotTo(BeNil())
	please.Expect(cmd.Director).NotTo(BeNil())
}

func setSomeOMVars(t *testing.T) {
	t.Helper()
	t.Setenv("OM_TARGET", "https://pcf.example.com")
	t.Setenv("OM_USERNAME", "banana")
	t.Setenv("OM_PASSWORD", "orange")
	t.Setenv("BOSH_ALL_PROXY", "ssh+socks5://ubuntu@pcf.example.com:22?private-key=private-key.key")
}

func TestCacheCompiledReleases_Execute_all_releases_are_already_compiled(t *testing.T) {
	please := NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: component.NewReleaseSources(&component.S3ReleaseSource{
			Bucket: "compiled-releases",
		}),
	})).NotTo(HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ReleaseLock{
			{
				Name:         "banana",
				Version:      "2.0.0",
				RemoteSource: "compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",
				SHA1:         "fake-checksum",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	})).NotTo(HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.0"}]}`, nil)

	deployment := new(boshdirFakes.FakeDeployment)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.FindDeploymentReturns(deployment, nil)

	releaseStorage := new(component_fakes.ReleaseUploader)
	releaseStorage.GetMatchedReleaseCalls(fakeCacheData)

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheReleases{
		FS:     fs,
		Logger: logger,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (component.ReleaseUploader, error) {
			return releaseStorage, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	setSomeOMVars(t)

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "compiled-releases",
	})

	// check

	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(output.String()).To(ContainSubstring("cache already contains releases"))
}

func TestCacheCompiledReleases_Execute_all_releases_are_already_cached(t *testing.T) {
	please := NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: component.NewReleaseSources(&component.S3ReleaseSource{
			Bucket: "compiled-releases",
		}),
	})).NotTo(HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ReleaseLock{
			{
				Name:    "orange",
				Version: "1.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "orange-1.0.0",

				SHA1: "fake-checksum",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	})).NotTo(HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.0"}]}`, nil)

	deployment := new(boshdirFakes.FakeDeployment)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.FindDeploymentReturns(deployment, nil)

	releaseStorage := new(component_fakes.ReleaseUploader)
	releaseStorage.GetMatchedReleaseCalls(fakeCacheData)

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheReleases{
		FS:     fs,
		Logger: logger,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (component.ReleaseUploader, error) {
			return releaseStorage, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	setSomeOMVars(t)

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "compiled-releases",
	})

	// check

	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(output.String()).To(ContainSubstring("cache already contains releases"))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(component.Lock{
		Name: "orange", Version: "1.0.0",
		SHA1:         "fake-checksum",
		RemoteSource: "cached-compiled-releases",
		RemotePath:   "orange-1.0.0-alpine-9.0.0",
	}))
}

// this test covers
//   - an export, download, upload, and lock of a non-cached release
//   - an update the kilnfile with a non-locked release cached in the database
//     (the release is cached on s3 but not set in the lock file)
//   - ignoring a locked and cached release
//     (the release is cached on the s3 bucket and the lock already has that value in it)
func TestCacheCompiledReleases_Execute_when_one_release_is_cached_another_is_already_compiled_and_another_is_already_locked(t *testing.T) {
	please := NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: component.NewReleaseSources(
			&component.S3ReleaseSource{
				Identifier:   "cached-compiled-releases",
				Publishable:  true,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
			&component.S3ReleaseSource{
				Identifier:   "new-releases",
				Publishable:  false,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			}),
	})).NotTo(HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ReleaseLock{
			{
				Name:    "orange",
				Version: "1.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "orange-1.0.0",

				SHA1: "fake-checksum",
			},
			{
				Name:    "banana",
				Version: "2.0.0",

				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",

				SHA1: "fake-checksum",
			},
			{
				Name:    "lemon",
				Version: "3.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "lemon-3.0.0",

				SHA1: "fake-checksum",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	})).NotTo(HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.0"}]}`, nil)

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
	bosh.FindReleaseStub = func(slug director.ReleaseSlug) (director.Release, error) {
		switch slug.Name() {
		default:
			panic(fmt.Errorf("FindReleaseStub input not handled: %#v", slug))
		case "lemon":
			return &boshdirFakes.FakeRelease{
				PackagesStub: func() ([]director.Package, error) {
					return []director.Package{{CompiledPackages: []director.CompiledPackage{{Stemcell: director.NewOSVersionSlug("alpine", "9.0.0")}}}}, nil
				},
			}, nil
		}
	}

	releaseStorage := new(component_fakes.ReleaseUploader)
	releaseStorage.GetMatchedReleaseCalls(fakeCacheData)

	var uploadedRelease bytes.Buffer
	releaseStorage.UploadReleaseCalls(func(_ context.Context, _ *log.Logger, _ component.Spec, reader io.Reader) (component.Lock, error) {
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

	cmd := commands.CacheReleases{
		FS:     fs,
		Logger: logger,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (component.ReleaseUploader, error) {
			return releaseStorage, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	setSomeOMVars(t)

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(releaseStorage.GetMatchedReleaseCallCount()).To(Equal(3))
	please.Expect(bosh.DownloadResourceUncheckedCallCount()).To(Equal(1))

	requestedID, _ := bosh.DownloadResourceUncheckedArgsForCall(0)
	please.Expect(requestedID).To(Equal("some-blob-id"))

	please.Expect(output.String()).To(ContainSubstring("1 release needs to be exported and cached"))
	please.Expect(output.String()).To(ContainSubstring("lemon/3.0.0 compiled with alpine/9.0.0 not found in cache"))
	please.Expect(output.String()).To(ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(output.String()).To(ContainSubstring("exporting lemon"))
	please.Expect(output.String()).To(ContainSubstring("downloading lemon"))
	please.Expect(output.String()).To(ContainSubstring("uploading lemon"))
	please.Expect(output.String()).To(ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))

	please.Expect(uploadedRelease.String()).To(Equal(string(releaseInBlobstore)))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(component.Lock{
		Name:         "lemon",
		Version:      "3.0.0",
		SHA1:         "012ed191f1d07c14bbcbbc0423d0de1c56757348",
		RemoteSource: "cached-compiled-releases",
		RemotePath:   "lemon-3.0.0-alpine-9.0.0",
	}))
}

// this test covers
// - when a release is compiled with a stemcell that is not the one in the Kilnfile.lock (aka the compilation target)
// - the deployment succeeds because the stemcell major lines are the same/compatible
// - export release returns a broken bosh release because we requested the wrong compilation target and the director didn't have the source code necessarily to re-compile against the requested stemcell
// - (ideally bosh export-release should return an error but in this case it doesn't, so we are just checking for a release with the correct stemcell before downloading a bad one)
func TestCacheCompiledReleases_Execute_when_a_release_is_not_compiled_with_the_correct_stemcell(t *testing.T) {
	please := NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: component.NewReleaseSources(
			&component.S3ReleaseSource{
				Identifier:   "cached-compiled-releases",
				Publishable:  true,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
			&component.S3ReleaseSource{
				Identifier:   "new-releases",
				Publishable:  false,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
		),
	})).NotTo(HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ReleaseLock{
			{
				Name:    "banana",
				Version: "2.0.0",

				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-5.5.5",

				SHA1: "fake-checksum",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "8.0.0",
		},
	})).NotTo(HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "8.0.0"}]}`, nil)

	deployment := new(boshdirFakes.FakeDeployment)
	deployment.ExportReleaseReturns(director.ExportReleaseResult{}, nil)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.FindDeploymentReturns(deployment, nil)
	bosh.FindReleaseStub = func(slug director.ReleaseSlug) (director.Release, error) {
		switch slug.Name() {
		default:
			panic(fmt.Errorf("FindReleaseStub input not handled: %#v", slug))
		case "banana":
			return &boshdirFakes.FakeRelease{
				PackagesStub: func() ([]director.Package, error) {
					return make([]director.Package, 1), nil
				},
			}, nil
		}
	}

	releaseStorage := new(component_fakes.ReleaseUploader)
	releaseStorage.GetMatchedReleaseCalls(fakeCacheData)

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheReleases{
		FS:     fs,
		Logger: logger,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (component.ReleaseUploader, error) {
			return releaseStorage, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	setSomeOMVars(t)

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(err).To(MatchError(ContainSubstring("not found on bosh director")))

	please.Expect(bosh.DownloadResourceUncheckedCallCount()).To(Equal(0))
	please.Expect(bosh.HasReleaseCallCount()).To(Equal(0))
	please.Expect(bosh.FindReleaseCallCount()).To(Equal(1))

	{
		requestedReleaseSlug := bosh.FindReleaseArgsForCall(0)
		please.Expect(requestedReleaseSlug.Name()).To(Equal("banana"))
		please.Expect(requestedReleaseSlug.Version()).To(Equal("2.0.0"))
	}

	please.Expect(output.String()).To(ContainSubstring("1 release needs to be exported and cached"))
	please.Expect(output.String()).To(ContainSubstring("banana/2.0.0 compiled with alpine/8.0.0 not found in cache"))
	please.Expect(output.String()).To(ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(output.String()).NotTo(ContainSubstring("exporting lemon"))
	please.Expect(output.String()).NotTo(ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(component.Lock{
		Name:    "banana",
		Version: "2.0.0",

		RemoteSource: "cached-compiled-releases",
		RemotePath:   "banana-2.0.0-alpine-5.5.5",

		SHA1: "fake-checksum",
	}), "it should not override the in-correct element in the Kilnfile.lock")
}

// this test covers
// - when a release does not contain packages
func TestCacheCompiledReleases_Execute_when_a_release_has_no_packages(t *testing.T) {
	please := NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: component.NewReleaseSources(
			&component.S3ReleaseSource{
				Identifier:   "cached-compiled-releases",
				Publishable:  true,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
			&component.S3ReleaseSource{
				Identifier:   "new-releases",
				Publishable:  false,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
		),
		Releases: []cargo.ReleaseSpec{
			{
				Name: "banana",
			},
		},
	})).NotTo(HaveOccurred())
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
		Releases: []cargo.ReleaseLock{
			{
				Name:    "banana",
				Version: "2.0.0",

				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-5.5.5",

				SHA1: "fake-checksum",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "8.0.0",
		},
	})).NotTo(HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "8.0.0"}]}`, nil)

	deployment := new(boshdirFakes.FakeDeployment)
	deployment.ExportReleaseReturns(director.ExportReleaseResult{SHA1: "sha256:7dd4f2f077e449b47215359e8020c0b6c81e184d2c614486246cb8f70cac7a70"}, nil)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.DownloadResourceUncheckedCalls(func(_ string, writer io.Writer) error {
		_, _ = writer.Write([]byte("greetings"))
		return nil
	})
	bosh.FindDeploymentReturns(deployment, nil)
	bosh.FindReleaseStub = func(slug director.ReleaseSlug) (director.Release, error) {
		switch slug.Name() {
		default:
			panic(fmt.Errorf("FindReleaseStub input not handled: %#v", slug))
		case "banana":
			return &boshdirFakes.FakeRelease{
				PackagesStub: func() ([]director.Package, error) {
					return make([]director.Package, 0), nil
				},
			}, nil
		}
	}

	releaseStorage := new(component_fakes.ReleaseUploader)
	releaseStorage.GetMatchedReleaseCalls(fakeCacheData)
	releaseStorage.UploadReleaseStub = func(_ context.Context, _ *log.Logger, spec cargo.ReleaseSpec, reader io.Reader) (cargo.ReleaseLock, error) {
		l := spec.Lock()
		l.RemotePath = "BANANA.tgz"
		l.RemoteSource = "BASKET"
		return l, nil
	}

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	cmd := commands.CacheReleases{
		FS:     fs,
		Logger: logger,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (component.ReleaseUploader, error) {
			return releaseStorage, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	setSomeOMVars(t)

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(bosh.DownloadResourceUncheckedCallCount()).To(Equal(1))
	please.Expect(bosh.HasReleaseCallCount()).To(Equal(0))
	please.Expect(bosh.FindReleaseCallCount()).To(Equal(1))

	{
		requestedReleaseSlug := bosh.FindReleaseArgsForCall(0)
		please.Expect(requestedReleaseSlug.Name()).To(Equal("banana"))
		please.Expect(requestedReleaseSlug.Version()).To(Equal("2.0.0"))
	}

	please.Expect(output.String()).To(ContainSubstring("1 release needs to be exported and cached"))
	please.Expect(output.String()).To(ContainSubstring("banana/2.0.0 compiled with alpine/8.0.0 not found in cache"))
	please.Expect(output.String()).To(ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(output.String()).To(ContainSubstring("oes not have any packages"))
	please.Expect(output.String()).To(ContainSubstring("exporting banana"))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(component.Lock{
		Name:    "banana",
		Version: "2.0.0",

		RemoteSource: "BASKET",
		RemotePath:   "BANANA.tgz",

		SHA1: "fake-checksum",
	}), "it should not override the in-correct element in the Kilnfile.lock")

	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(output.String()).To(ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))
}

func TestCacheCompiledReleases_Execute_staged_and_lock_stemcells_are_not_the_same(t *testing.T) {
	please := NewWithT(t)

	// setup

	fs := memfs.New()

	please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
		ReleaseSources: component.NewReleaseSources(
			&component.S3ReleaseSource{
				Identifier:   "cached-compiled-releases",
				Publishable:  true,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
			&component.S3ReleaseSource{
				Identifier:   "new-releases",
				Publishable:  false,
				PathTemplate: "{{.Release}}-{{.Version}}.tgz",
			},
		),
	})).NotTo(HaveOccurred())
	initialLock := cargo.KilnfileLock{
		Releases: []component.Lock{
			{
				Name:    "orange",
				Version: "1.0.0",

				RemoteSource: "new-releases",
				RemotePath:   "orange-1.0.0",

				SHA1: "fake-checksum",
			},
			{
				Name:    "banana",
				Version: "2.0.0",

				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",

				SHA1: "fake-checksum",
			},
			{
				Name:         "lemon",
				Version:      "3.0.0",
				SHA1:         "fake-checksum",
				RemoteSource: "new-releases",
				RemotePath:   "lemon-3.0.0",
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "alpine",
			Version: "9.0.0",
		},
	}
	please.Expect(fsWriteYAML(fs, "Kilnfile.lock", initialLock)).NotTo(HaveOccurred())

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": "9.0.1"}]}`, nil)

	releaseCache := new(component_fakes.ReleaseUploader)
	releaseCache.GetMatchedReleaseCalls(fakeCacheData)

	bosh := new(boshdirFakes.FakeDirector)

	cmd := commands.CacheReleases{
		FS: fs,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (component.ReleaseUploader, error) {
			return releaseCache, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}

	setSomeOMVars(t)

	// run

	err := cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(releaseCache.GetMatchedReleaseCallCount()).To(Equal(0))
	please.Expect(bosh.DownloadResourceUncheckedCallCount()).To(Equal(0))
	please.Expect(err).To(MatchError(Equal("staged stemcell (alpine 9.0.1) and lock stemcell (alpine 9.0.0) do not match")))

	var updatedLock cargo.KilnfileLock
	please.Expect(fsReadYAML(fs, "Kilnfile.lock", &updatedLock)).NotTo(HaveOccurred())
	please.Expect(updatedLock).To(Equal(initialLock))
}

func fakeCacheData(_ context.Context, _ *log.Logger, spec component.Spec) (component.Lock, error) {
	switch spec.Lock() {
	case component.Lock{Name: "orange", Version: "1.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
		return component.Lock{
			Name: "orange", Version: "1.0.0",
			SHA1:         "fake-checksum",
			RemoteSource: "cached-compiled-releases",
			RemotePath:   "orange-1.0.0-alpine-9.0.0",
		}, nil
	case component.Lock{Name: "banana", Version: "2.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
		return component.Lock{
			Name: "banana", Version: "2.0.0",
			SHA1:         "fake-checksum",
			RemoteSource: "cached-compiled-releases",
			RemotePath:   "banana-2.0.0-alpine-9.0.0",
		}, nil
	case component.Lock{Name: "lemon", Version: "3.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"},
		component.Lock{Name: "banana", Version: "2.0.0", StemcellOS: "alpine", StemcellVersion: "8.0.0"}:
		return component.Lock{}, component.ErrNotFound
	}

	panic(fmt.Sprintf("unexpected spec %#v", spec))
}
