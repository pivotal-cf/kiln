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

var _ = Describe("CarvelReBake", func() {
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

	Describe("Usage", func() {
		It("returns usage information", func() {
			usage := command.Usage()
			Expect(usage.ShortDescription).To(Equal("re-bakes a Carvel tile from a bake record"))
			Expect(usage.Description).To(ContainSubstring("bake record"))
		})
	})

	Describe("Execute", func() {
		When("required arguments are missing", func() {
			It("returns an error when no bake record is provided", func() {
				err := command.Execute([]string{
					"--output-file", "/tmp/out.pivotal",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exactly one bake record argument"))
			})

			It("returns an error when output-file is not provided", func() {
				err := command.Execute([]string{"some-record.json"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("output-file"))
			})
		})

		When("the bake record file does not exist", func() {
			It("returns an error", func() {
				err := command.Execute([]string{
					"--output-file", "/tmp/out.pivotal",
					"/nonexistent/record.json",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read bake record"))
			})
		})

		When("the bake record has a mismatched source revision", func() {
			var (
				inputPath  string
				recordPath string
			)

			BeforeEach(func() {
				var err error
				inputPath, err = os.MkdirTemp("", "rebake-test-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

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

				record := bake.Record{
					SourceRevision: "0000000000000000000000000000000000000000",
					Version:        "0.1.1",
					TileDirectory:  inputPath,
				}
				buf, err := json.Marshal(record)
				Expect(err).NotTo(HaveOccurred())

				recordPath = filepath.Join(filepath.Dir(inputPath), "record.json")
				err = os.WriteFile(recordPath, buf, 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
			})

			It("returns a source revision mismatch error", func() {
				err := command.Execute([]string{
					"--output-file", filepath.Join(filepath.Dir(inputPath), "out.pivotal"),
					recordPath,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("source revision"))
			})
		})

		When("a valid bake record and mock Artifactory are provided", func() {
			var (
				inputPath  string
				outputPath string
				recordPath string
				server     *httptest.Server
			)

			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed — skipping rebake integration test")
				}
				if !kilnInstalled() {
					Skip("kiln CLI not installed — skipping rebake integration test")
				}

				var err error
				inputPath, err = os.MkdirTemp("", "rebake-happy-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				gitCmd := func(args ...string) {
					cmd := exec.Command("git", args...)
					cmd.Dir = inputPath
					out, err := cmd.CombinedOutput()
					ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, out)
				}

				gitCmd("init")
				gitCmd("add", ".")
				gitCmd("commit", "-m", "initial commit")

				b := carvel.NewBaker()
				b.SetWriter(GinkgoWriter)
				Expect(b.Bake(inputPath)).To(Succeed())
				tarball, err := b.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())
				tarballData, err := os.ReadFile(tarball)
				Expect(err).NotTo(HaveOccurred())

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
				remotePath := "/test-repo/bosh-releases/k8s-tile-test/k8s-tile-test-0.1.1.tgz"
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
						Name:         "k8s-tile-test",
						Version:      "0.1.1",
						RemotePath:   "bosh-releases/k8s-tile-test/k8s-tile-test-0.1.1.tgz",
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
