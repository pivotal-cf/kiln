package component_test

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"

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

		calledUnexpectedEndpoint *atomic.Bool

		releasesDirectory string
	)
	BeforeEach(func() {
		calledUnexpectedEndpoint = new(atomic.Bool)
		calledUnexpectedEndpoint.Store(false)
		source = new(component.ArtifactoryReleaseSource)
		releasesDirectory = must(os.MkdirTemp("", "releases"))
		config = cargo.ReleaseSourceConfig{}

		config.Repo = "basket"
		config.PathTemplate = "bosh-releases/{{.StemcellOS}}/{{.StemcellVersion}}/{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz"
		config.Username = correctUsername
		config.Password = correctPassword
		config.ID = "some-mango-tree"

		artifactoryRouter = httprouter.New()
		artifactoryRouter.RedirectTrailingSlash = false
		artifactoryRouter.NotFound = http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			calledUnexpectedEndpoint.Store(true)
			log.Printf("handler on fake artifactory server not found not found for request: %s %s", req.Method, req.URL)
		})
	})
	JustBeforeEach(func() {
		server = httptest.NewServer(artifactoryRouter)
		config.ArtifactoryHost = server.URL
		source = component.NewArtifactoryReleaseSource(config)
		source.Client = server.Client()
	})
	AfterEach(func() {
		server.Close()
		_ = os.RemoveAll(releasesDirectory)
	})

	Describe("read operations", func() {
		BeforeEach(func() {
			artifactoryRouter.Handler(http.MethodHead, "/artifactory/basket/bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
				res.Header().Set("x-checksum-sha1", "some-sha")
				res.WriteHeader(http.StatusOK)
			})))
			artifactoryRouter.Handler(http.MethodGet, "/artifactory/basket/bosh-releases/smoothie/9.9/mango/", applyMiddleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
				res.WriteHeader(http.StatusOK)
				// language=html
				_, _ = io.WriteString(res, `<!DOCTYPE html>
<html>
<head>
	<title>Index of 🥭</title>
</head>
<body>
	<h1>Index of tanzu-application-services-generic-local/shared-releases</h1>
	<pre>Name           Last modified      Size</pre><hr/>
	<pre>
		<a href="../">../</a>
		<a href="mango-1.0.0-smoothie-9.9.tgz">mango-1.0.0-smoothie-9.9.tgz</a>	18-Apr-2023 14:05  162.89 MB
		<a href="mango-2.1.0-smoothie-9.9.tgz">mango-2.1.0-smoothie-9.9.tgz</a>	18-Apr-2023 14:05  162.89 MB
		<a href="mango-2.3.4-smoothie-9.9.tgz">mango-2.3.4-smoothie-9.9.tgz</a>	18-Apr-2023 14:05  162.89 MB
		<a href="mango-3.0.0-smoothie-9.9.tgz">mango-3.0.0-smoothie-9.9.tgz</a>	18-Apr-2023 14:05  162.89 MB
	</pre>
	<hr/>
</body>
</html>`)
			})))
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
		When("the server has the a file at the expected path", func() {
			It("resolves the lock from the spec", func() { // testing GetMatchedRelease
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
					SHA1:         "some-sha",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				}))

				By("only calling expected endpoints", func() {
					Expect(calledUnexpectedEndpoint.Load()).To(BeFalse())
				})
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
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
					SHA1:            "some-sha",
					RemotePath:      "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource:    "some-mango-tree",
				}))

				By("only calling expected endpoints", func() {
					Expect(calledUnexpectedEndpoint.Load()).To(BeFalse())
				})
			})

			It("downloads the release", func() { // teesting DownloadRelease
				By("calling FindReleaseVersion")
				local, resultErr := source.DownloadRelease(releasesDirectory, component.Lock{
					Name:         "mango",
					Version:      "2.3.4",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				})

				Expect(resultErr).NotTo(HaveOccurred())
				Expect(local.LocalPath).To(BeAnExistingFile())

				By("only calling expected endpoints", func() {
					Expect(calledUnexpectedEndpoint.Load()).To(BeFalse())
				})
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

	When("not behind the corporate firewall", func() {
		JustBeforeEach(func() {
			source.Client.Transport = dnsFailure{}
		})
		Describe("GetMatchedRelease", func() {
			It("returns a helpful message", func() {
				_, resultErr := source.GetMatchedRelease(component.Spec{
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
				_, resultErr := source.FindReleaseVersion(component.Spec{
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
				_, resultErr := source.DownloadRelease(releasesDirectory, component.Lock{
					Name:         "mango",
					Version:      "2.3.4",
					RemotePath:   "bosh-releases/smoothie/9.9/mango/mango-2.3.4-smoothie-9.9.tgz",
					RemoteSource: "some-mango-tree",
				})
				Expect(resultErr).To(HaveOccurred())
				Expect(resultErr.Error()).To(ContainSubstring("vpn"))
			})
		})
		Describe("UploadRelease", func() {
			It("returns a helpful message", func() {
				_, resultErr := source.UploadRelease(component.Spec{
					Name:            "mango",
					Version:         "2.3.4",
					StemcellOS:      "smoothie",
					StemcellVersion: "9.9",
				}, bytes.NewBuffer(nil))
				Expect(resultErr).To(HaveOccurred())
				Expect(resultErr.Error()).To(ContainSubstring("vpn"))
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
	return nil, &net.DNSError{Err: "lemon sorbet"}
}

func must[T any](value T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return value
}
