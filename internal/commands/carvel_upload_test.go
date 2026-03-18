package commands_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"

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
				Expect(err.Error()).To(ContainSubstring("Kilnfile not found"))
			})
		})

		When("valid Kilnfile is provided with a mock Artifactory", func() {
			var (
				inputPath string
				server    *httptest.Server
			)

			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}

				var err error
				inputPath, err = os.MkdirTemp("", "upload-test-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
				}))

				kf := cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{
						{
							Type:            "artifactory",
							ArtifactoryHost: server.URL,
							Repo:            "test-repo",
							Username:        "user",
							Password:        "pass",
							PathTemplate:    "bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz",
						},
					},
				}
				kfData, err := yaml.Marshal(&kf)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(filepath.Join(inputPath, "Kilnfile"), kfData, 0644)
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
				err = yaml.Unmarshal(lockData, &lock)
				Expect(err).NotTo(HaveOccurred())
				Expect(lock.Releases).To(HaveLen(1))
				Expect(lock.Releases[0].Name).To(Equal("k8s-tile-test"))
				Expect(lock.Releases[0].Version).To(Equal("0.1.1"))
				Expect(lock.Releases[0].SHA1).NotTo(BeEmpty())
				Expect(lock.Releases[0].RemotePath).To(ContainSubstring("k8s-tile-test"))
				Expect(lock.Releases[0].RemoteSource).To(Equal("artifactory"))
			})
		})
	})
})
