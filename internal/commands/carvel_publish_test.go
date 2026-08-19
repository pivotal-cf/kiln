package commands_test

import (
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands"
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

	})
})
