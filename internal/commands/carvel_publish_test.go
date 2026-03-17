package commands_test

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/pkg/bake"
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

		When("no Kilnfile.lock exists", func() {
			It("returns an error telling the user to run upload first", func() {
				tmpDir, err := os.MkdirTemp("", "publish-no-lock-*")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.RemoveAll(tmpDir) }()

				err = command.Execute([]string{
					"--source-directory", tmpDir,
					"--output-file", filepath.Join(tmpDir, "out.pivotal"),
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Kilnfile.lock not found"))
				Expect(err.Error()).To(ContainSubstring("kiln carvel upload"))
			})
		})

		When("--final flag is used with a lockfile", func() {
			var (
				inputPath  string
				outputPath string
			)

			BeforeEach(func() {
				if !boshInstalled() {
					Skip("bosh CLI not installed - skipping integration test")
				}
				if !kilnInstalled() {
					Skip("kiln CLI not installed - skipping integration test")
				}

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

				// Simulate `kiln carvel upload`: bake to produce a BOSH release,
				// then create a Kilnfile.lock pointing to the cached tarball.
				baker := carvel.NewBaker()
				baker.SetWriter(GinkgoWriter)
				err = baker.Bake(inputPath)
				Expect(err).NotTo(HaveOccurred())

				tarball, err := baker.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())

				cachedTarball := filepath.Join(filepath.Dir(inputPath), "cached-release.tgz")
				copyFile(tarball, cachedTarball)

				lf := models.CarvelLockfile{
					Release: models.CarvelReleaseLock{
						Name:       "k8s-tile-test",
						Version:    "0.1.1",
						RemotePath: cachedTarball,
						SHA256:     "test-sha",
					},
				}
				Expect(lf.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"))).To(Succeed())

				outputPath = filepath.Join(filepath.Dir(inputPath), "output.pivotal")
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
			})

			It("bakes the tile and creates a bake record", func() {
				err := command.Execute([]string{
					"--source-directory", inputPath,
					"--output-file", outputPath,
					"--final",
					"--verbose",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())

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
				err = json.Unmarshal(recordData, &record)
				Expect(err).NotTo(HaveOccurred())
				Expect(record.Version).To(Equal("0.1.1"))
				Expect(record.SourceRevision).NotTo(BeEmpty())
				Expect(record.FileChecksum).NotTo(BeEmpty())
			})
		})
	})
})
