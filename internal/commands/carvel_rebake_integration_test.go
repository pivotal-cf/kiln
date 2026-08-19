//go:build integration

package commands_test

import (
	"encoding/json"
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

	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/pkg/bake"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v3"
)

// Integration specs for CarvelReBake; drive the real bosh CLI end to end.
var _ = Describe("CarvelReBake (integration)", func() {
	var (
		outLogger *log.Logger
		errLogger *log.Logger
		command   commands.CarvelReBake
	)

	BeforeEach(func() {
		outLogger = log.New(GinkgoWriter, "", 0)
		errLogger = log.New(GinkgoWriter, "", 0)
		command = commands.NewCarvelReBake(outLogger, errLogger)
	})

	Describe("Execute", func() {
		When("a valid bake record and mock Artifactory are provided", func() {
			var (
				inputPath  string
				outputPath string
				recordPath string
				server     *httptest.Server
			)

			BeforeEach(func() {

				var err error
				inputPath, err = os.MkdirTemp("", "rebake-happy-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				gitCmd := func(args ...string) {
					// Pin the identity so commits work on a CI runner with no
					// global git user configured.
					ident := []string{"-c", "user.name=test", "-c", "user.email=test@test.com"}
					cmd := exec.Command("git", append(ident, args...)...)
					cmd.Dir = inputPath
					out, err := cmd.CombinedOutput()
					ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, out)
				}

				gitCmd("init")
				gitCmd("add", ".")
				gitCmd("commit", "-m", "initial commit")

				b := carvel.NewBaker()
				b.SetWriter(GinkgoWriter)
				Expect(b.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, carvel.BakeOptions{})).To(Succeed())
				tarball, err := b.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())
				tarballData, err := os.ReadFile(tarball)
				Expect(err).NotTo(HaveOccurred())
				releaseVersion := b.GetReleaseVersion()

				var (
					mu    sync.Mutex
					blobs = make(map[string][]byte)
				)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					key := strings.TrimPrefix(r.URL.Path, "/artifactory")
					switch r.Method {
					case http.MethodPut:
						body, _ := io.ReadAll(r.Body)
						mu.Lock()
						blobs[key] = body
						mu.Unlock()
						w.WriteHeader(http.StatusCreated)
					case http.MethodGet:
						mu.Lock()
						data, ok := blobs[key]
						mu.Unlock()
						if !ok {
							http.Error(w, "not found", http.StatusNotFound)
							return
						}
						w.Header().Set("Content-Type", "application/gzip")
						_, _ = w.Write(data)
					}
				}))

				// Pre-load the mock with the tarball at the expected path
				remotePath := "/test-repo/bosh-releases/k8s-tile-test-pkg/k8s-tile-test-pkg-" + releaseVersion + ".tgz"
				blobs[remotePath] = tarballData

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

				lock := cargo.KilnfileLock{
					Releases: []cargo.BOSHReleaseTarballLock{{
						Name:         "k8s-tile-test-pkg",
						Version:      releaseVersion,
						RemotePath:   "bosh-releases/k8s-tile-test-pkg/k8s-tile-test-pkg-" + releaseVersion + ".tgz",
						RemoteSource: "artifactory",
					}},
					Stemcell: cargo.Stemcell{OS: "ubuntu-jammy", Version: "1.446"},
				}
				lockData, err := yaml.Marshal(&lock)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), lockData, 0644)).To(Succeed())

				gitCmd("add", ".")
				gitCmd("commit", "-m", "add kilnfiles")

				sha := strings.TrimSpace(func() string {
					cmd := exec.Command("git", "rev-parse", "HEAD")
					cmd.Dir = inputPath
					out, _ := cmd.Output()
					return string(out)
				}())

				record := bake.Record{
					SourceRevision: sha,
					Version:        "0.1.1",
					TileDirectory:  inputPath,
				}
				buf, err := json.Marshal(record)
				Expect(err).NotTo(HaveOccurred())

				recordPath = filepath.Join(filepath.Dir(inputPath), "record.json")
				Expect(os.WriteFile(recordPath, buf, 0644)).To(Succeed())

				outputPath = filepath.Join(filepath.Dir(inputPath), "output.pivotal")
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
				if server != nil {
					server.Close()
				}
			})

			It("re-bakes successfully from the bake record", func() {
				err := command.Execute([]string{
					"--output-file", outputPath,
					"--verbose",
					recordPath,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())
			})
		})
	})
})
