package component_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/julienschmidt/httprouter"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("interacting with BOSH releases on Artifactory", func() {
	const (
		correctUsername = "kim"
		correctPassword = "mango_rice!"
	)

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
	})
	JustBeforeEach(func() {
		server = httptest.NewServer(artifactoryRouter)
		config.ArtifactoryHost = server.URL
		source = component.NewArtifactoryReleaseSource(config)
	})
	AfterEach(func() {
		server.Close()
		_ = os.RemoveAll(releasesDirectory)
	})

	Describe("read operations", func() {
		When("the server has the a file at the expected path", func() {
			BeforeEach(func() {
				requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

				artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusOK)
					// language=json
					_, _ = io.WriteString(res, `{"checksums": {"sha1":  "some-sha"}}`)
				})))
				artifactoryRouter.Handler(http.MethodGet, "/api/storage/basket/bosh-releases/smoothie/9.9/mango", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusOK)
					// language=json
					_, _ = io.WriteString(res, `{"children": [{"uri": "/mango-2.3.4-smoothie-9.9.tgz", "folder": false}]}`)
				}), requireAuth))
				artifactoryRouter.Handler(http.MethodGet, "/artifactory/basket/bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
					res.WriteHeader(http.StatusOK)
					f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
					if err != nil {
						log.Fatal("failed to open some release test artifact")
					}
					defer closeAndIgnoreError(f)
					_, _ = io.Copy(res, f)
				}) /* put middleware here */))
			})

			It("resolves the lock from the spec", func() { // testing GetMatchedRelease
				source.ID = "some-mango-tree"
				source.ReleaseSourceConfig.ID = "why-are-we-not-using-this-field"
				resultLock, resultErr := source.GetMatchedRelease(component.Spec{
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
				})

				Expect(resultErr).NotTo(HaveOccurred())
				Expect(resultLock).To(Equal(component.Lock{
					Name:    "mango",
					Version: "2.3.4",
					// StemcellOS:      "smoothie",
					// StemcellVersion: "9.9",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				}))
			})

			It("finds the bosh release", func() { // testing FindReleaseVersion
				resultLock, resultErr := source.FindReleaseVersion(component.Spec{
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
				}, false)

				Expect(resultErr).NotTo(HaveOccurred())
				Expect(resultLock).To(Equal(component.Lock{
					Name:    "mango",
					Version: "2.3.4",
					// StemcellOS:      "smoothie",
					// StemcellVersion: "9.9",
					SHA1:         "some-sha",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				}))
			})

			It("downloads the release", func() {
				source.ID = "some-mango-tree"
				source.ReleaseSourceConfig.ID = "why-are-we-not-using-this-field"

				By("calling FindReleaseVersion")
				local, resultErr := source.DownloadRelease(releasesDirectory, component.Lock{
					Name:         "mango",
					Version:      "2.3.4",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				})

				Expect(resultErr).NotTo(HaveOccurred())
				Expect(local.LocalPath).To(BeAnExistingFile())
			})
		})
	})
	When("uploading releases", func() { // testing UploadRelease
		var serverReleasesDirectory string
		BeforeEach(func() {
			serverReleasesDirectory = must(os.MkdirTemp("", "artifactory_server"))

			requireAuth := requireBasicAuthMiddleware(correctUsername, correctPassword)

			artifactoryRouter.Handler(http.MethodPut, "/artifactory/basket/bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				fileName := path.Base(req.URL.Path)
				filePath := filepath.Join(serverReleasesDirectory, fileName)
				f, err := os.Create(filePath)
				if err != nil {
					log.Fatal("failed to create file in", filePath)
				}
				defer closeAndIgnoreError(f)
				defer closeAndIgnoreError(req.Body)
				_, _ = io.Copy(f, req.Body)
				res.WriteHeader(http.StatusCreated)
			}), requireAuth))
		})
		AfterEach(func() {
			_ = os.RemoveAll(serverReleasesDirectory)
		})

		It("it uploads the file to the server", func() { // testing UploadRelease
			f, err := os.Open(filepath.Join("testdata", "some-release.tgz"))
			Expect(err).NotTo(HaveOccurred())
			defer closeAndIgnoreError(f)

			By("calling UploadRelease")
			resultLock, resultErr := source.UploadRelease(component.Spec{
				Name:            "mango",
				Version:         "2.3.4",
				StemcellOS:      "smoothie",
				StemcellVersion: "9.9",
			}, f)

			Expect(resultErr).NotTo(HaveOccurred())
			Expect(resultLock).To(Equal(component.Lock{
				Name:    "mango",
				Version: "2.3.4",
				// StemcellOS:      "smoothie",
				// StemcellVersion: "9.9",
				// SHA1:         "some-sha",
				RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
				RemoteSource: "some-mango-tree",
			}))
			Expect(filepath.Join(serverReleasesDirectory, "mango-2.3.4-smoothie-9.9.tgz")).To(BeAnExistingFile())
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

func must[T any](value T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
}
