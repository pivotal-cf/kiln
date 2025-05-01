package component_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("interacting with BOSH releases on Artifactory", func() {
	const (
		correctUsername = "kim"
		correctPassword = "mango_rice!"
	)

	type ApiStorageChildren struct {
		Path       string `json:"path"`
		Repo       string `json:"repo"`
		Name       string `json:"name"`
		ActualSha1 string `json:"actual_sha1"`
		Folder     bool   `json:"folder"`
	}
	type ApiStorageListing struct {
		Children []ApiStorageChildren `json:"results"`
	}

	var (
		source            *component.ArtifactoryReleaseSource
		config            cargo.ReleaseSourceConfig
		server            *httptest.Server
		artifactoryRouter *httprouter.Router

		releasesDirectory string
	)
	BeforeEach(func() {
		source = new(component.ArtifactoryReleaseSource)

		releasesDirectory = must(os.MkdirTemp("", "releases"))

		config = cargo.ReleaseSourceConfig{}

		config.Repo = "basket"
		config.PathTemplate = "bosh-releases/{{.StemcellOS}}/{{.StemcellVersion}}/{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz"
		config.Username = correctUsername
		config.Password = correctPassword
		config.ID = "some-mango-tree"

		artifactoryRouter = httprouter.New()
		artifactoryRouter.NotFound = http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			log.Fatalf("handler on fake artifactory server not found not found for request: %s %s", req.Method, req.URL)
		})
		artifactoryRouter.Handler(http.MethodGet, "/artifactory/api/system/ping", http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
			res.WriteHeader(http.StatusOK)
		}))
		artifactoryRouter.Handler(http.MethodGet, "/api/system/version", http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusOK)
			_, err := res.Write([]byte(`{"version": "7.0.0", "revision": "0"}`))
			Expect(err).ToNot(HaveOccurred())
		}))
	})
	JustBeforeEach(func() {
		logger := log.New(GinkgoWriter, "", 0)
		server = httptest.NewServer(artifactoryRouter)
		config.ArtifactoryHost = server.URL
		source = component.NewArtifactoryReleaseSource(config, logger)
		source.Client = server.Client()
	})
	AfterEach(func() {
		server.Close()
		_ = os.RemoveAll(releasesDirectory)
	})

	Describe("read operations", func() {
		When("release files exist", func() {
			When("the stemcell info is only in the path", func() {
				BeforeEach(func() {
					config.PathTemplate = "bosh-releases/{{.StemcellOS}}/{{.StemcellVersion}}/{{.Name}}/{{.Name}}-{{.Version}}.tgz"
					requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

					apiStorageListing := ApiStorageListing{}
					for _, filename := range []string{
						"invalid",
						"mango-2.3.3.tgz",
						"mango-2.3.4-build.1.tgz",
						"mango-2.3.4.tgz",
						"mango-2.3.5.tgz",
						"mango-2.3.4-build.2.tgz",
						"mango-2.3.5-notices.zip",
						"notices-mango-2.3.5.zip",
						"orange-10.0.0.tgz",
					} {
						apiStoragePath := fmt.Sprintf("/api/storage/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, apiStoragePath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							// language=json
							_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
						}), requireAuth))

						downloadPath := fmt.Sprintf("/artifactory/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, downloadPath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
							if err != nil {
								log.Fatal("failed to open some release test artifact")
							}
							defer closeAndIgnoreError(f)
							_, _ = io.Copy(res, f)
						}), requireAuth))

						apiStorageListing.Children = append(apiStorageListing.Children, ApiStorageChildren{
							Path:       "bosh-releases/smoothie/9.9/mango",
							Name:       filename,
							ActualSha1: "some-sha",
						})
					}

					apiStorageListingBytes, err := json.Marshal(apiStorageListing)
					Expect(err).NotTo(HaveOccurred())

					artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))
					artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))
				})
				It("resolves the lock from the spec", func() { // testing GetMatchedRelease
					resultLock, resultErr := source.GetMatchedRelease(cargo.BOSHReleaseTarballSpecification{
						Name:            "mango",
						Version:         "2.3.4",
						StemcellOS:      "smoothie",
						StemcellVersion: "9.9",
					})

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
						Name:    "mango",
						Version: "2.3.4",
						// StemcellOS:      "smoothie",
						// StemcellVersion: "9.9",
						RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4.tgz",
						RemoteSource: "some-mango-tree",
						SHA1:         "some-sha",
					}))
				})

				It("finds the bosh release", func() { // testing FindReleaseVersion
					resultLock, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
						Name:            "mango",
						Version:         "*",
						StemcellOS:      "smoothie",
						StemcellVersion: "9.9",
					}, false)

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
						Name:    "mango",
						Version: "2.3.5",
						//StemcellOS:      "smoothie",
						//StemcellVersion: "9.9",
						SHA1:         "some-sha",
						RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.5.tgz",
						RemoteSource: "some-mango-tree",
					}))
				})

				It("downloads the release", func() { // teesting DownloadRelease
					By("calling FindReleaseVersion")
					local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
						Name:            "mango",
						Version:         "2.3.4",
						StemcellOS:      "smoothie",
						StemcellVersion: "9.9",
						RemotePath:      "bosh-releases/smoothie/9.9/mango/mango-2.3.4.tgz",
						RemoteSource:    "some-mango-tree",
					})

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(local.LocalPath).To(BeAnExistingFile())
				})
				When("the server URL ends in /artifactory", func() {
					JustBeforeEach(func() {
						logger := log.New(GinkgoWriter, "", 0)
						config.ArtifactoryHost = server.URL + "/artifactory"
						source = component.NewArtifactoryReleaseSource(config, logger)
						source.Client = server.Client()
					})

					It("downloads the release", func() {
						By("calling FindReleaseVersion")
						local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
							Name:            "mango",
							Version:         "2.3.4",
							StemcellOS:      "smoothie",
							StemcellVersion: "9.9",
							RemotePath:      "bosh-releases/smoothie/9.9/mango/mango-2.3.4.tgz",
							RemoteSource:    "some-mango-tree",
						})

						Expect(resultErr).NotTo(HaveOccurred())
						Expect(local.LocalPath).To(BeAnExistingFile())
					})
				})
				When("the server URL is malformed", func() {
					JustBeforeEach(func() {
						logger := log.New(GinkgoWriter, "", 0)
						config.ArtifactoryHost = ":improper-url/formatting"
						source = component.NewArtifactoryReleaseSource(config, logger)
						source.Client = server.Client()
					})
					It("returns an error", func() {
						local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
							Name:            "mango",
							Version:         "2.3.4",
							StemcellOS:      "smoothie",
							StemcellVersion: "9.9",
							RemotePath:      "bosh-releases/smoothie/9.9/mango/mango-2.3.4.tgz",
							RemoteSource:    "some-mango-tree",
						})

						Expect(resultErr).To(HaveOccurred())
						Expect(local).To(Equal(component.Local{}))
					})
				})
			})
			When("there is no stemcell configured", func() {
				BeforeEach(func() {
					config.PathTemplate = "bosh-releases/{{.Name}}-{{.Version}}.tgz"
					requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)
					apiStorageListing := ApiStorageListing{}
					for _, filename := range []string{
						"invalid",
						"mango-2.3.3.tgz",
						"mango-2.3.4-build.1.tgz",
						"mango-2.3.4.tgz",
						"mango-2.3.4-build.2.tgz",
						"mango-2.3.5-notices.zip",
						"notices-mango-2.3.5.zip",
						"orange-10.0.0.tgz",
					} {
						apiStoragePath := fmt.Sprintf("/api/storage/basket/bosh-releases/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, apiStoragePath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							// language=json
							_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
						}), requireAuth))

						downloadPath := fmt.Sprintf("/artifactory/basket/bosh-releases/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, downloadPath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
							if err != nil {
								log.Fatal("failed to open some release test artifact")
							}
							defer closeAndIgnoreError(f)
							_, _ = io.Copy(res, f)
						}), requireAuth))

						apiStorageListing.Children = append(apiStorageListing.Children, ApiStorageChildren{
							Path:       "bosh-releases",
							Name:       filename,
							ActualSha1: "some-sha",
						})
					}

					apiStorageListingBytes, err := json.Marshal(apiStorageListing)
					Expect(err).NotTo(HaveOccurred())

					artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))

					artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))
				})

				It("resolves the lock from the spec", func() { // testing GetMatchedRelease
					resultLock, resultErr := source.GetMatchedRelease(cargo.BOSHReleaseTarballSpecification{
						Name:    "mango",
						Version: "2.3.4",
					})

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
						Name:         "mango",
						Version:      "2.3.4",
						RemotePath:   "bosh-releases/mango-2.3.4.tgz",
						RemoteSource: "some-mango-tree",
						SHA1:         "some-sha",
					}))
				})

				It("finds the bosh release", func() { // testing FindReleaseVersion
					resultLock, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
						Name:    "mango",
						Version: "2.3.4",
						//StemcellOS:      "smoothie",
						//StemcellVersion: "9.9",
					}, false)

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
						Name:    "mango",
						Version: "2.3.4",
						// StemcellOS:      "smoothie",
						// StemcellVersion: "9.9",
						SHA1:         "some-sha",
						RemotePath:   "bosh-releases/mango-2.3.4.tgz",
						RemoteSource: "some-mango-tree",
					}))
				})

				It("downloads the release", func() { // testing DownloadRelease
					By("calling FindReleaseVersion")
					local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
						Name:         "mango",
						Version:      "2.3.4",
						RemotePath:   "bosh-releases/mango-2.3.4.tgz",
						RemoteSource: "some-mango-tree",
					})

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(local.LocalPath).To(BeAnExistingFile())
				})
				When("the server URL ends in /artifactory", func() {
					JustBeforeEach(func() {
						logger := log.New(GinkgoWriter, "", 0)
						config.ArtifactoryHost = server.URL + "/artifactory"
						source = component.NewArtifactoryReleaseSource(config, logger)
						source.Client = server.Client()
					})

					It("downloads the release", func() {
						By("calling FindReleaseVersion")
						local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
							Name:         "mango",
							Version:      "2.3.4",
							RemotePath:   "bosh-releases/mango-2.3.4.tgz",
							RemoteSource: "some-mango-tree",
						})

						Expect(resultErr).NotTo(HaveOccurred())
						Expect(local.LocalPath).To(BeAnExistingFile())
					})
				})
				When("the server URL is malformed", func() {
					JustBeforeEach(func() {
						logger := log.New(GinkgoWriter, "", 0)
						config.ArtifactoryHost = ":improper-url/formatting"
						source = component.NewArtifactoryReleaseSource(config, logger)
						source.Client = server.Client()
					})
					It("returns an error", func() {
						local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
							Name:         "mango",
							Version:      "2.3.4",
							RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4.tgz",
							RemoteSource: "some-mango-tree",
						})

						Expect(resultErr).To(HaveOccurred())
						Expect(local).To(Equal(component.Local{}))
					})
				})
			})
			When("there are pre-releases and full releases", func() {
				BeforeEach(func() {
					requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

					apiStorageListing := ApiStorageListing{}
					for _, filename := range []string{
						"mango-2.3.4-build.1-smoothie-9.9.tgz",
						"mango-2.3.4-smoothie-9.9.tgz",
						"mango-2.3.4-build.2-smoothie-9.9.tgz",
						"mango-3.0.0-build.1-smoothie-9.9.tgz",
						"mango-4.0.0-build.1.tgz",
					} {
						apiStoragePath := fmt.Sprintf("/api/storage/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, apiStoragePath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							// language=json
							_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
						}), requireAuth))

						downloadPath := fmt.Sprintf("/artifactory/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, downloadPath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
							if err != nil {
								log.Fatal("failed to open some release test artifact")
							}
							defer closeAndIgnoreError(f)
							_, _ = io.Copy(res, f)
						}), requireAuth))

						apiStorageListing.Children = append(apiStorageListing.Children, ApiStorageChildren{
							Path:       "bosh-releases/smoothie/9.9/mango",
							Name:       filename,
							ActualSha1: "some-sha",
						})
					}

					apiStorageListingBytes, err := json.Marshal(apiStorageListing)
					Expect(err).NotTo(HaveOccurred())

					artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))

					artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))
				})
				When("we allow pre-releases", func() {
					It("finds the latest version", func() {
						resultLock, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
							Name:            "mango",
							Version:         ">0-0",
							StemcellOS:      "smoothie",
							StemcellVersion: "9.9",
						}, false)

						Expect(resultErr).NotTo(HaveOccurred())
						Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
							Name:    "mango",
							Version: "3.0.0-build.1",
							// StemcellOS:      "smoothie",
							// StemcellVersion: "9.9",
							SHA1:         "some-sha",
							RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-3.0.0-build.1-smoothie-9.9.tgz",
							RemoteSource: "some-mango-tree",
						}))
					})
				})
				When("we disallow pre-releases", func() {
					It("finds the latest bosh version", func() { // testing FindReleaseVersion
						resultLock, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
							Name:            "mango",
							Version:         "*",
							StemcellOS:      "smoothie",
							StemcellVersion: "9.9",
						}, false)

						Expect(resultErr).NotTo(HaveOccurred())
						Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
							Name:    "mango",
							Version: "2.3.4",
							// StemcellOS:      "smoothie",
							// StemcellVersion: "9.9",
							SHA1:         "some-sha",
							RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
							RemoteSource: "some-mango-tree",
						}))
					})

				})
			})
			When("there are only pre-releases", func() {
				BeforeEach(func() {
					requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

					apiStorageListing := ApiStorageListing{}
					for _, filename := range []string{
						"mango-2.3.4-build.1-smoothie-9.9.tgz",
						"mango-2.3.4-build.3-smoothie-9.9.tgz",
						"mango-2.3.4-build.2-smoothie-9.9.tgz",
					} {
						apiStoragePath := fmt.Sprintf("/api/storage/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, apiStoragePath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							// language=json
							_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
						}), requireAuth))

						downloadPath := fmt.Sprintf("/artifactory/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, downloadPath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
							if err != nil {
								log.Fatal("failed to open some release test artifact")
							}
							defer closeAndIgnoreError(f)
							_, _ = io.Copy(res, f)
						}), requireAuth))

						apiStorageListing.Children = append(apiStorageListing.Children, ApiStorageChildren{
							Path:       "bosh-releases/smoothie/9.9/mango",
							Name:       filename,
							ActualSha1: "some-sha",
						})
					}

					apiStorageListingBytes, err := json.Marshal(apiStorageListing)
					Expect(err).NotTo(HaveOccurred())

					artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))

					artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))
				})
				When("we allow pre-releases", func() {
					It("finds the latest version", func() { // testing FindReleaseVersion
						resultLock, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
							Name:            "mango",
							Version:         ">=0.0.0-build.0",
							StemcellOS:      "smoothie",
							StemcellVersion: "9.9",
						}, false)

						Expect(resultErr).NotTo(HaveOccurred())
						Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
							Name:         "mango",
							Version:      "2.3.4-build.3",
							SHA1:         "some-sha",
							RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-build.3-smoothie-9.9.tgz",
							RemoteSource: "some-mango-tree",
						}))
					})
				})
				When("dont allow pre-releases", func() {
					It("returns ErrNotFound", func() {
						_, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
							Name:            "mango",
							Version:         "*",
							StemcellOS:      "smoothie",
							StemcellVersion: "9.9",
						}, false)

						Expect(resultErr).To(HaveOccurred())
						Expect(component.IsErrNotFound(resultErr)).To(BeTrue())
					})

				})
			})
			When("there are pre and full releases and invalid files", func() {
				BeforeEach(func() {
					requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

					apiStorageListing := ApiStorageListing{}
					for _, filename := range []string{
						"",
						"invalid",
						"mango-2.3.3-smoothie-9.9.tgz",
						"mango-2.3.4-build.1-smoothie-9.9.tgz",
						"mango-2.3.4-smoothie-9.9.tgz",
						"mango-2.3.4-build.2-smoothie-9.9.tgz",
						"mango-2.3.5-notices.zip",
						"notices-mango-2.3.5.zip",
						"orange-10.0.0-smoothie-9.9.tgz",
					} {
						apiStoragePath := fmt.Sprintf("/api/storage/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, apiStoragePath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							// language=json
							_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
						}), requireAuth))

						downloadPath := fmt.Sprintf("/artifactory/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
						artifactoryRouter.Handler(http.MethodGet, downloadPath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
							res.WriteHeader(http.StatusOK)
							f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
							if err != nil {
								log.Fatal("failed to open some release test artifact")
							}
							defer closeAndIgnoreError(f)
							_, _ = io.Copy(res, f)
						}), requireAuth))

						apiStorageListing.Children = append(apiStorageListing.Children, ApiStorageChildren{
							Path:       "bosh-releases/smoothie/9.9/mango",
							Name:       filename,
							ActualSha1: "some-sha",
						})
					}

					apiStorageListingBytes, err := json.Marshal(apiStorageListing)
					Expect(err).NotTo(HaveOccurred())

					artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))

					artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.Writer.Write(res, apiStorageListingBytes)
					}), requireAuth))
				})
				It("resolves the lock from the spec", func() { // testing GetMatchedRelease
					resultLock, resultErr := source.GetMatchedRelease(cargo.BOSHReleaseTarballSpecification{
						Name:            "mango",
						Version:         "2.3.4",
						StemcellOS:      "smoothie",
						StemcellVersion: "9.9",
					})

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
						Name:    "mango",
						Version: "2.3.4",
						// StemcellOS:      "smoothie",
						// StemcellVersion: "9.9",
						RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
						RemoteSource: "some-mango-tree",
						SHA1:         "some-sha",
					}))
				})

				It("finds the bosh release", func() { // testing FindReleaseVersion
					resultLock, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
						Name:            "mango",
						Version:         "2.3.4",
						StemcellOS:      "smoothie",
						StemcellVersion: "9.9",
					}, false)

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(resultLock).To(Equal(cargo.BOSHReleaseTarballLock{
						Name:    "mango",
						Version: "2.3.4",
						// StemcellOS:      "smoothie",
						// StemcellVersion: "9.9",
						SHA1:         "some-sha",
						RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
						RemoteSource: "some-mango-tree",
					}))
				})

				It("downloads the release", func() { // teesting DownloadRelease
					By("calling FindReleaseVersion")
					local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
						Name:         "mango",
						Version:      "2.3.4",
						RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
						RemoteSource: "some-mango-tree",
					})

					Expect(resultErr).NotTo(HaveOccurred())
					Expect(local.LocalPath).To(BeAnExistingFile())
				})
				When("the server URL ends in /artifactory", func() {
					JustBeforeEach(func() {
						logger := log.New(GinkgoWriter, "", 0)
						config.ArtifactoryHost = server.URL + "/artifactory"
						source = component.NewArtifactoryReleaseSource(config, logger)
						source.Client = server.Client()
					})

					It("downloads the release", func() {
						By("calling FindReleaseVersion")
						local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
							Name:         "mango",
							Version:      "2.3.4",
							RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
							RemoteSource: "some-mango-tree",
						})

						Expect(resultErr).NotTo(HaveOccurred())
						Expect(local.LocalPath).To(BeAnExistingFile())
					})
				})
				When("the server URL is malformed", func() {
					JustBeforeEach(func() {
						logger := log.New(GinkgoWriter, "", 0)
						config.ArtifactoryHost = ":improper-url/formatting"
						source = component.NewArtifactoryReleaseSource(config, logger)
						source.Client = server.Client()
					})
					It("returns an error", func() {
						local, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
							Name:         "mango",
							Version:      "2.3.4",
							RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
							RemoteSource: "some-mango-tree",
						})

						Expect(resultErr).To(HaveOccurred())
						Expect(local).To(Equal(component.Local{}))
					})
				})
			})
		})
	})

	When("not behind the corporate firewall", func() {
		BeforeEach(func() {
			requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)
			artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
				res.WriteHeader(http.StatusOK)
			}), requireAuth))
		})
		JustBeforeEach(func() {
			source.Client.Transport = dnsFailure{}
		})
		Describe("GetMatchedRelease", func() {
			It("returns a helpful message", func() {
				_, resultErr := source.GetMatchedRelease(cargo.BOSHReleaseTarballSpecification{
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
				})
				Expect(resultErr).To(HaveOccurred())
				Expect(resultErr.Error()).To(ContainSubstring("vpn"))
			})
		})
		Describe("FindReleaseVersion", func() {
			It("returns a helpful message", func() {
				_, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
				}, false)
				Expect(resultErr).To(HaveOccurred())
				Expect(resultErr.Error()).To(ContainSubstring("vpn"))
			})
		})
		Describe("DownloadRelease", func() {
			It("returns a helpful message", func() {
				_, resultErr := source.DownloadRelease(releasesDirectory, cargo.BOSHReleaseTarballLock{
					Name:         "mango",
					Version:      "2.3.4",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				})
				Expect(resultErr).To(HaveOccurred())
				Expect(resultErr.Error()).To(ContainSubstring("vpn"))
			})
		})
	})

	When("a bosh release is not found", func() {
		When("there are no files", func() {
			BeforeEach(func() {
				apiStorageListing := ApiStorageListing{}
				apiStorageListingBytes, err := json.Marshal(apiStorageListing)
				Expect(err).NotTo(HaveOccurred())
				requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

				artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusNotFound)
					// language=json
					_, _ = io.Writer.Write(res, apiStorageListingBytes)
				}), requireAuth))
				artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusOK)
					// language=json
					_, _ = io.Writer.Write(res, apiStorageListingBytes)
				}), requireAuth))
			})
			It("returns ErrNotFound", func() {
				_, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
					Name:            "missing-release",
					Version:         "1.2.3",
					StemcellOS:      "ubuntu-jammy",
					StemcellVersion: "1.234",
				}, false)
				Expect(component.IsErrNotFound(resultErr)).To(BeTrue())

				_, resultErr = source.GetMatchedRelease(cargo.BOSHReleaseTarballSpecification{
					Name:            "missing-release",
					Version:         "1.2.3",
					StemcellOS:      "ubuntu-jammy",
					StemcellVersion: "1.234",
				})
				Expect(component.IsErrNotFound(resultErr)).To(BeTrue())
			})
		})
		When("there are invalid files", func() {
			BeforeEach(func() {
				requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

				apiStorageListing := ApiStorageListing{}
				for _, filename := range []string{
					"",
					"invalid",
					"mango-2.3.4-invalid.zip",
					"mango-2.3.4-invalid-9.9.zip",
					"mango-2.3.4-invalid-smoothie-9.9.zip",
					"invalid-mango-2.3.4.zip",
				} {
					apiStoragePath := fmt.Sprintf("/api/storage/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
					artifactoryRouter.Handler(http.MethodGet, apiStoragePath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						// language=json
						_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
					}), requireAuth))

					downloadPath := fmt.Sprintf("/artifactory/basket/bosh-releases/smoothie/9.9/mango/%s", filename)
					artifactoryRouter.Handler(http.MethodGet, downloadPath, applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
						res.WriteHeader(http.StatusOK)
						f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
						if err != nil {
							log.Fatal("failed to open some release test artifact")
						}
						defer closeAndIgnoreError(f)
						_, _ = io.Copy(res, f)
					}), requireAuth))

					apiStorageListing.Children = append(apiStorageListing.Children, ApiStorageChildren{
						Path:       "bosh-releases/smoothie/9.9/mango",
						Name:       filename,
						ActualSha1: "some-sha",
					})
				}

				apiStorageListingBytes, err := json.Marshal(apiStorageListing)
				Expect(err).NotTo(HaveOccurred())

				artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusOK)
					// language=json
					_, _ = io.Writer.Write(res, apiStorageListingBytes)
				}), requireAuth))

				artifactoryRouter.Handler(http.MethodPost, "/api/search/aql", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusOK)
					// language=json
					_, _ = io.Writer.Write(res, apiStorageListingBytes)
				}), requireAuth))
			})
			It("returns ErrNotFound", func() { // testing FindReleaseVersion
				_, resultErr := source.FindReleaseVersion(cargo.BOSHReleaseTarballSpecification{
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
				}, false)

				Expect(resultErr).To(HaveOccurred())
				Expect(component.IsErrNotFound(resultErr)).To(BeTrue())
			})
		})
	})
})

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}

func requireBasicAuthMiddleware(expectedUsername, expectedPassword string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			username, password, ok := req.BasicAuth()
			if !ok {
				http.Error(res, "auth not set", http.StatusUnauthorized)
				return
			}
			if expectedUsername != username {
				http.Error(res, "username does not match", http.StatusUnauthorized)
				return
			}
			if expectedPassword != password {
				http.Error(res, "password does not match", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}

func applyMiddleware(endpoint http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	h := endpoint
	for _, mw := range middleware {
		h = mw(h)
	}
	return h
}

type dnsFailure struct{}

func (dnsFailure) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &net.DNSError{Err: "some error"}
}

func must[T any](value T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return value
}
