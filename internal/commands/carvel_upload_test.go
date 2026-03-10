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
	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"github.com/pivotal-cf/kiln/internal/commands"
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
		When("required arguments are missing", func() {
			It("returns an error when artifactory-host is not provided", func() {
				err := command.Execute([]string{
					"--artifactory-repo", "some-repo",
					"--artifactory-username", "user",
					"--artifactory-password", "pass",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("artifactory-host"))
			})
		})

		When("valid arguments are provided with a mock Artifactory", func() {
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

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
				}))
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
				if server != nil {
					server.Close()
				}
			})

			It("uploads the BOSH release and writes a lockfile", func() {
				err := command.Execute([]string{
					"--source-directory", inputPath,
					"--artifactory-host", server.URL,
					"--artifactory-repo", "test-repo",
					"--artifactory-username", "user",
					"--artifactory-password", "pass",
					"--verbose",
				})
				Expect(err).NotTo(HaveOccurred())

				lockfilePath := filepath.Join(inputPath, "Kilnfile.lock")
				Expect(lockfilePath).To(BeAnExistingFile())

				lf, err := models.ReadCarvelLockfile(lockfilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(lf.Release.Name).To(Equal("k8s-tile-test"))
				Expect(lf.Release.Version).To(Equal("0.1.1"))
				Expect(lf.Release.SHA256).NotTo(BeEmpty())
				Expect(lf.Release.RemotePath).To(ContainSubstring("k8s-tile-test"))
			})
		})
	})
})
