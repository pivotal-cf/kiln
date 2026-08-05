package commands_test

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands"
)

func boshInstalled() bool {
	_, err := exec.LookPath("bosh")
	return err == nil
}

func kilnInstalled() bool {
	_, err := exec.LookPath("kiln")
	return err == nil
}

var _ = Describe("CarvelBake", func() {
	var (
		outLogger *log.Logger
		errLogger *log.Logger
		command   commands.CarvelBake
	)

	BeforeEach(func() {
		outLogger = log.New(GinkgoWriter, "", 0)
		errLogger = log.New(GinkgoWriter, "", 0)
		command = commands.NewCarvelBake(outLogger, errLogger)
	})

	Describe("Usage", func() {
		It("returns usage information", func() {
			usage := command.Usage()
			Expect(usage.ShortDescription).To(Equal("bakes a Carvel/Kubernetes tile"))
			Expect(usage.Description).To(ContainSubstring("Carvel/Kubernetes tile"))
		})
	})

	Describe("Execute", func() {
		var (
			inputPath  string
			outputPath string
		)

		BeforeEach(func() {
			var err error
			inputPath, err = os.MkdirTemp("", "testinput-*")
			Expect(err).NotTo(HaveOccurred())
			inputPath += "/tile"
			err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
			Expect(err).NotTo(HaveOccurred())

			// create an initial git commit in the input directory
			cmds := []*exec.Cmd{
				exec.Command("git", "init"),
				exec.Command("git", "add", "."),
				exec.Command("git", "-c", "user.name=test", "-c", "user.email=test@test.com", "commit", "-m", "initial commit"),
			}
			for _, cmd := range cmds {
				cmd.Dir = inputPath
				out, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
			}

			outputPath = filepath.Join(inputPath, "output.pivotal")
		})

		AfterEach(func() {
			if inputPath != "" {
				_ = os.RemoveAll(filepath.Dir(inputPath))
			}
		})

		When("required arguments are missing", func() {
			It("returns an error when output-file is not provided", func() {
				err := command.Execute([]string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("output-file"))
			})
		})

		When("valid arguments are provided", func() {
			It("successfully bakes a tile", func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
				if !kilnInstalled() {
					Skip("kiln CLI not installed - skipping integration test")
				}
				err := command.Execute([]string{
					"--source-directory", inputPath,
					"--output-file", outputPath,
					"--verbose",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())
			})
		})

		When("a Kilnfile.lock is present but --from-lockfile is not set", func() {
			It("successfully bakes a tile from source (ignoring the lockfile for the tile's own release)", func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
				if !kilnInstalled() {
					Skip("kiln CLI not installed - skipping integration test")
				}

				err := os.WriteFile(filepath.Join(inputPath, "Kilnfile"), []byte(`---
release_sources: []
`), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), []byte(`---
releases:
- name: some-other-release
  version: 1.2.3
`), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = command.Execute([]string{
					"--source-directory", inputPath,
					"--output-file", outputPath,
					"--verbose",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())
			})
		})

		When("a Kilnfile.lock is present but fails to parse", func() {
			It("fails loudly instead of silently baking without additional_releases support", func() {
				err := os.WriteFile(filepath.Join(inputPath, "Kilnfile"), []byte(`---
release_sources: []
`), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), []byte(`---
releases:
- name: cf-cli
  version: "1.0.0"
 - name: smoke-tests
   version: "2.0.0"
`), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = command.Execute([]string{
					"--source-directory", inputPath,
					"--output-file", outputPath,
					"--verbose",
				})
				Expect(err).To(HaveOccurred())
				Expect(outputPath).NotTo(BeAnExistingFile())
			})
		})
	})
})
