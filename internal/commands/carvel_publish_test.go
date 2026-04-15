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

var _ = Describe("CarvelPublish", func() {
	var (
		outLogger *log.Logger
		errLogger *log.Logger
		command   commands.CarvelPublish
	)

	BeforeEach(func() {
		outLogger = log.New(GinkgoWriter, "", 0)
		errLogger = log.New(GinkgoWriter, "", 0)
		command = commands.NewCarvelPublish(outLogger, errLogger)
	})

	Describe("Usage", func() {
		It("returns usage information", func() {
			usage := command.Usage()
			Expect(usage.ShortDescription).To(Equal("publishes a Carvel/Kubernetes tile"))
			Expect(usage.Description).To(ContainSubstring("bake record"))
		})
	})

	Describe("Execute", func() {
		When("required arguments are missing", func() {
			It("returns an error when output-file is not provided", func() {
				err := command.Execute([]string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("output-file"))
			})
		})

		When("no Kilnfile exists", func() {
			It("returns an error telling the user to run upload first", func() {
				tmpDir, err := os.MkdirTemp("", "publish-no-kilnfile-*")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.RemoveAll(tmpDir) }()

				err = command.Execute([]string{
					"--source-directory", tmpDir,
					"--output-file", filepath.Join(tmpDir, "out.pivotal"),
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find Kilnfile"))
				Expect(err.Error()).To(ContainSubstring("kiln carvel upload"))
			})
		})

		When("--final flag is used with a round-trip mock Artifactory", func() {
			var (
				inputPath  string
				outputPath string
				server     *httptest.Server
				mu         sync.Mutex
				blobs      map[string][]byte
				getCount   int
			)

			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
				if !kilnInstalled() {
					Skip("kiln CLI not installed - skipping integration test")
				}

				blobs = make(map[string][]byte)
				getCount = 0

				var err error
				inputPath, err = os.MkdirTemp("", "publish-test-*")
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

				b := carvel.NewBaker()
				b.SetWriter(GinkgoWriter)
				Expect(b.Bake(inputPath)).To(Succeed())
				tarball, err := b.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())
				tarballData, err := os.ReadFile(tarball)
				Expect(err).NotTo(HaveOccurred())
				releaseVersion := b.GetReleaseVersion()

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
						data, found := blobs[key]
						getCount++
						mu.Unlock()
						if !found {
							http.Error(w, "not found", http.StatusNotFound)
							return
						}
						w.Header().Set("Content-Type", "application/gzip")
						_, _ = w.Write(data)
					}
				}))

				// Pre-load mock with the tarball (simulating a prior upload)
				remotePath := "/test-repo/bosh-releases/k8s-tile-test/k8s-tile-test-" + releaseVersion + ".tgz"
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
						Version:      releaseVersion,
						RemotePath:   "bosh-releases/k8s-tile-test/k8s-tile-test-" + releaseVersion + ".tgz",
						RemoteSource: "artifactory",
					}},
					Stemcell: cargo.Stemcell{OS: "ubuntu-jammy", Version: "1.446"},
				}
				lockData, err := yaml.Marshal(&lock)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), lockData, 0644)).To(Succeed())

				for _, cmd := range []*exec.Cmd{
					exec.Command("git", "add", "."),
					exec.Command("git", "commit", "-m", "add kilnfiles"),
				} {
					cmd.Dir = inputPath
					out, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
				}

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

			It("downloads the tarball, bakes the tile, and creates a bake record", func() {
				err := command.Execute([]string{
					"--source-directory", inputPath,
					"--output-file", outputPath,
					"--final",
					"--verbose",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())

				By("verifying the mock received a GET (download)")
				mu.Lock()
				Expect(getCount).To(BeNumerically(">=", 1), "publish must download from Artifactory")
				mu.Unlock()

				resolvedInput, resolveErr := filepath.EvalSymlinks(inputPath)
				if resolveErr != nil {
					resolvedInput = inputPath
				}

				recordsDir := filepath.Join(resolvedInput, "bake_records")
				Expect(recordsDir).To(BeADirectory())

				entries, err := os.ReadDir(recordsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				Expect(entries[0].Name()).To(Equal("0.1.1.json"))

				recordData, err := os.ReadFile(filepath.Join(recordsDir, "0.1.1.json"))
				Expect(err).NotTo(HaveOccurred())

				var record bake.Record
				Expect(json.Unmarshal(recordData, &record)).To(Succeed())
				Expect(record.Version).To(Equal("0.1.1"))
				Expect(record.SourceRevision).NotTo(BeEmpty())
				Expect(record.FileChecksum).NotTo(BeEmpty())
			})
		})
	})
})
