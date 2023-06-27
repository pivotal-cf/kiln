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
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/om"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ jhanda.Command = (*commands.CacheCompiledReleases)(nil)

func TestNewCacheCompiledReleases(t *testing.T) {
	please := NewWithT(t)
	cmd := commands.NewCacheCompiledReleases()
	please.Expect(cmd).NotTo(BeNil())
	please.Expect(cmd.Logger).NotTo(BeNil())
	please.Expect(cmd.FS).NotTo(BeNil())
	please.Expect(cmd.ReleaseSourceAndCache).NotTo(BeNil())
	please.Expect(cmd.OpsManager).NotTo(BeNil())
	please.Expect(cmd.Director).NotTo(BeNil())
}

type cacheCompiledReleasesTestData struct {
	cmd *commands.CacheCompiledReleases

	bosh           *boshdirFakes.FakeDirector
	deployment     *boshdirFakes.FakeDeployment
	opsManager     *fakes.OpsManagerReleaseCacheSource
	output         *bytes.Buffer
	releaseStorage *fakes.ReleaseStorage
}

const (
	releaseInBlobstore = `lemon-release-buffer`
)

func newCacheCompiledReleasesTestData(t *testing.T, kf cargo.Kilnfile, kl cargo.KilnfileLock, stagedStemcellVersion string) cacheCompiledReleasesTestData {
	t.Helper()

	fs := memfs.New()

	if err := fsWriteYAML(fs, "Kilnfile", kf); err != nil {
		t.Fatal(err)
	}
	if err := fsWriteYAML(fs, "Kilnfile.lock", kl); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	releaseStorage := new(fakes.ReleaseStorage)
	releaseStorage.GetMatchedReleaseCalls(func(spec cargo.BOSHReleaseTarballSpecification) (cargo.BOSHReleaseTarballLock, error) {
		switch spec.Lock() {
		case cargo.BOSHReleaseTarballLock{Name: "orange", Version: "1.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
			return cargo.BOSHReleaseTarballLock{
				Name: "orange", Version: "1.0.0",
				SHA1:         "fake-checksum",
				RemoteSource: "cached-compiled-releases",
				RemotePath:   "orange-1.0.0-alpine-9.0.0",
			}, nil
		case cargo.BOSHReleaseTarballLock{Name: "banana", Version: "2.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"}:
			return cargo.BOSHReleaseTarballLock{
				Name: "banana", Version: "2.0.0",
				SHA1:         "fake-checksum",
				RemoteSource: "cached-compiled-releases",
				RemotePath:   "banana-2.0.0-alpine-9.0.0",
			}, nil
		case cargo.BOSHReleaseTarballLock{Name: "lemon", Version: "3.0.0", StemcellOS: "alpine", StemcellVersion: "9.0.0"},
			cargo.BOSHReleaseTarballLock{Name: "banana", Version: "2.0.0", StemcellOS: "alpine", StemcellVersion: "8.0.0"}:
			return cargo.BOSHReleaseTarballLock{}, component.ErrNotFound
		}

		panic(fmt.Sprintf("unexpected spec %#v", spec))
	})

	opsManager := new(fakes.OpsManagerReleaseCacheSource)
	opsManager.GetStagedProductManifestReturns(fmt.Sprintf(`{"name": "cf-some-id", "stemcells": [{"os": "alpine", "version": %q}]}`, stagedStemcellVersion), nil)

	deployment := new(boshdirFakes.FakeDeployment)
	bosh := new(boshdirFakes.FakeDirector)
	bosh.FindDeploymentReturns(deployment, nil)

	cmd := commands.CacheCompiledReleases{
		FS:     fs,
		Logger: logger,
		ReleaseSourceAndCache: func(kilnfile cargo.Kilnfile, targetID string) (commands.ReleaseStorage, error) {
			return releaseStorage, nil
		},
		OpsManager: func(configuration om.ClientConfiguration) (commands.OpsManagerReleaseCacheSource, error) {
			return opsManager, nil
		},
		Director: func(configuration om.ClientConfiguration, provider om.GetBoshEnvironmentAndSecurityRootCACertificateProvider) (director.Director, error) {
			return bosh, nil
		},
	}
	return cacheCompiledReleasesTestData{
		cmd: &cmd,

		bosh:           bosh,
		output:         &output,
		deployment:     deployment,
		opsManager:     opsManager,
		releaseStorage: releaseStorage,
	}
}

func TestCacheCompiledReleases_Execute_all_releases_are_already_compiled(t *testing.T) {
	please := NewWithT(t)

	// setup

	test := newCacheCompiledReleasesTestData(t, cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				ID: "compiled-releases",
			},
		},
	}, cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
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
	}, "9.0.0")

	// run

	err := test.cmd.Execute([]string{
		"--upload-target-id", "compiled-releases",
	})

	// check

	please.Expect(test.output.String()).To(ContainSubstring("cache already contains releases"))
	please.Expect(err).NotTo(HaveOccurred())
}

func TestCacheCompiledReleases_Execute_all_releases_are_already_cached(t *testing.T) {
	please := NewWithT(t)

	// setup

	test := newCacheCompiledReleasesTestData(t, cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				ID: "compiled-releases",
			},
		},
	}, cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
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
	}, "9.0.0")

	// run

	err := test.cmd.Execute([]string{
		"--upload-target-id", "compiled-releases",
	})

	// check

	please.Expect(test.output.String()).To(ContainSubstring("cache already contains releases"))
	please.Expect(err).NotTo(HaveOccurred())

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(test.cmd.FS, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(cargo.BOSHReleaseTarballLock{
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
	// setup

	test := newCacheCompiledReleasesTestData(t, cargo.Kilnfile{
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
	}, cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
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
	}, "9.0.0")

	test.deployment.ExportReleaseReturns(director.ExportReleaseResult{
		BlobstoreID: "some-blob-id",
		SHA1:        fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(releaseInBlobstore))),
	}, nil)
	test.bosh.DownloadResourceUncheckedCalls(func(_ string, writer io.Writer) error {
		_, _ = writer.Write([]byte(releaseInBlobstore))
		return nil
	})
	test.bosh.FindReleaseStub = func(slug director.ReleaseSlug) (director.Release, error) {
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

	var uploadedRelease bytes.Buffer
	test.releaseStorage.UploadReleaseCalls(func(_ cargo.BOSHReleaseTarballSpecification, reader io.Reader) (cargo.BOSHReleaseTarballLock, error) {
		_, _ = io.Copy(&uploadedRelease, reader)
		return cargo.BOSHReleaseTarballLock{
			Name: "lemon", Version: "3.0.0",

			RemoteSource: "cached-compiled-releases",
			RemotePath:   "lemon-3.0.0-alpine-9.0.0",
			SHA1:         "012ed191f1d07c14bbcbbc0423d0de1c56757348",
		}, nil
	})
	test.releaseStorage.DownloadReleaseReturns(component.Local{}, fmt.Errorf("SO MUCH NOTHING"))

	// run

	err := test.cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check
	please := NewWithT(t)
	please.Expect(err).NotTo(HaveOccurred())
	please.Expect(test.releaseStorage.GetMatchedReleaseCallCount()).To(Equal(3))
	please.Expect(test.bosh.DownloadResourceUncheckedCallCount()).To(Equal(1))

	requestedID, _ := test.bosh.DownloadResourceUncheckedArgsForCall(0)
	please.Expect(requestedID).To(Equal("some-blob-id"))

	please.Expect(test.output.String()).To(ContainSubstring("1 release needs to be exported and cached"))
	please.Expect(test.output.String()).To(ContainSubstring("lemon/3.0.0 compiled with alpine/9.0.0 not found in cache"))
	please.Expect(test.output.String()).To(ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(test.output.String()).To(ContainSubstring("exporting lemon"))
	please.Expect(test.output.String()).To(ContainSubstring("downloading lemon"))
	please.Expect(test.output.String()).To(ContainSubstring("uploading lemon"))
	please.Expect(test.output.String()).To(ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))

	please.Expect(uploadedRelease.String()).To(Equal(releaseInBlobstore))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(test.cmd.FS, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(cargo.BOSHReleaseTarballLock{
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
// - (ideally bosh export-release should return an error but in this case it doesn't so we are just checking for a release with the correct stemcell before downloading a bad one)
func TestCacheCompiledReleases_Execute_when_a_release_is_not_compiled_with_the_correct_stemcell(t *testing.T) {
	please := NewWithT(t)

	// setup

	test := newCacheCompiledReleasesTestData(t, cargo.Kilnfile{
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
	}, cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
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
	}, "8.0.0")

	test.bosh.FindReleaseStub = func(slug director.ReleaseSlug) (director.Release, error) {
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

	// run

	err := test.cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(err).To(MatchError(ContainSubstring("not found on bosh director")))

	please.Expect(test.bosh.DownloadResourceUncheckedCallCount()).To(Equal(0))
	please.Expect(test.bosh.HasReleaseCallCount()).To(Equal(0))
	please.Expect(test.bosh.FindReleaseCallCount()).To(Equal(1))

	{
		requestedReleaseSlug := test.bosh.FindReleaseArgsForCall(0)
		please.Expect(requestedReleaseSlug.Name()).To(Equal("banana"))
		please.Expect(requestedReleaseSlug.Version()).To(Equal("2.0.0"))
	}

	please.Expect(test.output.String()).To(ContainSubstring("1 release needs to be exported and cached"))
	please.Expect(test.output.String()).To(ContainSubstring("banana/2.0.0 compiled with alpine/8.0.0 not found in cache"))
	please.Expect(test.output.String()).To(ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(test.output.String()).NotTo(ContainSubstring("exporting lemon"))
	please.Expect(test.output.String()).NotTo(ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(test.cmd.FS, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(cargo.BOSHReleaseTarballLock{
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
	test := newCacheCompiledReleasesTestData(t, cargo.Kilnfile{
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
		Releases: []cargo.BOSHReleaseTarballSpecification{
			{
				Name: "banana",
			},
		},
	}, cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
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
	}, "8.0.0")

	test.deployment.ExportReleaseReturns(director.ExportReleaseResult{SHA1: "sha256:7dd4f2f077e449b47215359e8020c0b6c81e184d2c614486246cb8f70cac7a70"}, nil)
	test.bosh.DownloadResourceUncheckedCalls(func(_ string, writer io.Writer) error {
		_, _ = writer.Write([]byte("greetings"))
		return nil
	})
	test.bosh.FindReleaseStub = func(slug director.ReleaseSlug) (director.Release, error) {
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
	test.releaseStorage.UploadReleaseStub = func(spec cargo.BOSHReleaseTarballSpecification, reader io.Reader) (cargo.BOSHReleaseTarballLock, error) {
		l := spec.Lock()
		l.RemotePath = "BANANA.tgz"
		l.RemoteSource = "BASKET"
		return l, nil
	}
	test.releaseStorage.DownloadReleaseReturns(component.Local{}, fmt.Errorf("SO MUCH NOTHING"))
	// run

	err := test.cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(test.bosh.DownloadResourceUncheckedCallCount()).To(Equal(1))
	please.Expect(test.bosh.HasReleaseCallCount()).To(Equal(0))
	please.Expect(test.bosh.FindReleaseCallCount()).To(Equal(1))

	{
		requestedReleaseSlug := test.bosh.FindReleaseArgsForCall(0)
		please.Expect(requestedReleaseSlug.Name()).To(Equal("banana"))
		please.Expect(requestedReleaseSlug.Version()).To(Equal("2.0.0"))
	}

	please.Expect(test.output.String()).To(ContainSubstring("1 release needs to be exported and cached"))
	please.Expect(test.output.String()).To(ContainSubstring("banana/2.0.0 compiled with alpine/8.0.0 not found in cache"))
	please.Expect(test.output.String()).To(ContainSubstring("exporting from bosh deployment cf-some-id"))
	please.Expect(test.output.String()).To(ContainSubstring("oes not have any packages"))
	please.Expect(test.output.String()).To(ContainSubstring("exporting banana"))

	var updatedKilnfile cargo.KilnfileLock
	please.Expect(fsReadYAML(test.cmd.FS, "Kilnfile.lock", &updatedKilnfile)).NotTo(HaveOccurred())
	please.Expect(updatedKilnfile.Releases).To(ContainElement(cargo.BOSHReleaseTarballLock{
		Name:    "banana",
		Version: "2.0.0",

		RemoteSource: "BASKET",
		RemotePath:   "BANANA.tgz",

		SHA1: "fake-checksum",
	}), "it should not override the in-correct element in the Kilnfile.lock")

	please.Expect(err).NotTo(HaveOccurred())

	please.Expect(test.output.String()).To(ContainSubstring("DON'T FORGET TO MAKE A COMMIT AND PR"))
}

func TestCacheCompiledReleases_Execute_staged_and_lock_stemcells_are_not_the_same(t *testing.T) {
	please := NewWithT(t)

	// setup

	initialLock := cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
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

	test := newCacheCompiledReleasesTestData(t, cargo.Kilnfile{
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
	}, initialLock, "9.0.1")

	// run

	err := test.cmd.Execute([]string{
		"--upload-target-id", "cached-compiled-releases",
	})

	// check

	please.Expect(test.releaseStorage.GetMatchedReleaseCallCount()).To(Equal(0))
	please.Expect(test.bosh.DownloadResourceUncheckedCallCount()).To(Equal(0))
	please.Expect(err).To(MatchError(Equal("staged stemcell (alpine 9.0.1) and lock stemcell (alpine 9.0.0) do not match")))

	var updatedLock cargo.KilnfileLock
	please.Expect(fsReadYAML(test.cmd.FS, "Kilnfile.lock", &updatedLock)).NotTo(HaveOccurred())
	please.Expect(updatedLock).To(Equal(initialLock))
}
