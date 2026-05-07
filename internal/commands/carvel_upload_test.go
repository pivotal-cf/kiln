package commands_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v3"
)

var _ = Describe("CarvelUpload", func() {
	var (
		outLogger *log.Logger
		errLogger *log.Logger
		command   commands.CarvelUpload
	)

	BeforeEach(func() {
		outLogger = log.New(GinkgoWriter, "", 0)
		errLogger = log.New(GinkgoWriter, "", 0)
		command = commands.NewCarvelUpload(outLogger, errLogger)
	})

	Describe("Usage", func() {
		It("returns usage information", func() {
			usage := command.Usage()
			Expect(usage.ShortDescription).To(Equal("uploads a Carvel BOSH release to Artifactory"))
			Expect(usage.Description).To(ContainSubstring("Artifactory"))
		})
	})

	Describe("Execute", func() {
		When("Kilnfile is missing", func() {
			It("returns an error", func() {
				tmpDir, err := os.MkdirTemp("", "upload-no-kilnfile-*")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.RemoveAll(tmpDir) }()

				err = command.Execute([]string{
					"--source-directory", tmpDir,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find Kilnfile"))
			})
		})

		When("valid Kilnfile is provided with a round-trip mock Artifactory", func() {
			var (
				inputPath      string
				server         *httptest.Server
				mu             sync.Mutex
				blobs          map[string][]byte
				authOK         bool
				putRequestURIs []string
			)

			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}

				blobs = make(map[string][]byte)
				authOK = false
				putRequestURIs = nil

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					u, p, ok := r.BasicAuth()
					if !ok || u != "user" || p != "pass" {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					key := strings.TrimPrefix(r.URL.Path, "/artifactory")
					switch r.Method {
					case http.MethodPut:
						body, _ := io.ReadAll(r.Body)
						mu.Lock()
						blobs[key] = body
						authOK = true
						putRequestURIs = append(putRequestURIs, r.RequestURI)
						mu.Unlock()
						w.WriteHeader(http.StatusCreated)
					case http.MethodGet:
						mu.Lock()
						data, found := blobs[key]
						mu.Unlock()
						if !found {
							http.Error(w, "not found", http.StatusNotFound)
							return
						}
						w.Header().Set("Content-Type", "application/gzip")
						_, _ = w.Write(data)
					}
				}))

				var err error
				inputPath, err = os.MkdirTemp("", "upload-test-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				kf := cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{{
						Type:            "artifactory",
						ArtifactoryHost: server.URL,
						Repo:            "test-repo",
						Username:        "user",
						Password:        "pass",
						PathTemplate:    "bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz",
					}},
				}
				kfData, err := yaml.Marshal(&kf)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile"), kfData, 0644)).To(Succeed())

				cmds := []*exec.Cmd{
					exec.Command("git", "init"),
					exec.Command("git", "add", "."),
					exec.Command("git", "commit", "-m", "initial commit"),
				}
				for _, cmd := range cmds {
					cmd.Dir = inputPath
					out, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
				}
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
				if server != nil {
					server.Close()
				}
			})

			It("uploads the BOSH release and writes a standard Kilnfile.lock", func() {
				err := command.Execute([]string{
					"--source-directory", inputPath,
					"--verbose",
				})
				Expect(err).NotTo(HaveOccurred())

				lockfilePath := filepath.Join(inputPath, "Kilnfile.lock")
				Expect(lockfilePath).To(BeAnExistingFile())

				lockData, err := os.ReadFile(lockfilePath)
				Expect(err).NotTo(HaveOccurred())

				var lock cargo.KilnfileLock
				Expect(yaml.Unmarshal(lockData, &lock)).To(Succeed())
				Expect(lock.Releases).To(HaveLen(1))
				Expect(lock.Releases[0].Name).To(Equal("k8s-tile-test"))
				Expect(lock.Releases[0].Version).To(HavePrefix("0.1.1+"))
				Expect(lock.Releases[0].Version).To(MatchRegexp(`^0\.1\.1\+[0-9a-f]{12}$`))
				Expect(lock.Releases[0].SHA1).NotTo(BeEmpty())
				Expect(lock.Releases[0].RemotePath).To(ContainSubstring("k8s-tile-test"))
				Expect(lock.Releases[0].RemotePath).To(ContainSubstring(lock.Releases[0].Version))
				Expect(lock.Releases[0].RemoteSource).To(Equal("artifactory"))

				By("verifying mock Artifactory received the PUT with Basic Auth")
				mu.Lock()
				Expect(authOK).To(BeTrue(), "upload must authenticate with Basic Auth")
				Expect(blobs).To(HaveLen(1), "exactly one blob should be stored")
				for _, data := range blobs {
					Expect(len(data)).To(BeNumerically(">", 0), "uploaded tarball must not be empty")
				}
				Expect(putRequestURIs).To(HaveLen(1))
				Expect(putRequestURIs[0]).To(ContainSubstring("%2B"),
					"PUT request URI must contain %2B for + characters")
				Expect(putRequestURIs[0]).NotTo(ContainSubstring("+"),
					"PUT request URI must not contain + characters")
				mu.Unlock()
			})
		})
	})
})
