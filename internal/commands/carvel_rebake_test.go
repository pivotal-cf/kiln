package commands_test

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/pkg/bake"
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
	})
})
