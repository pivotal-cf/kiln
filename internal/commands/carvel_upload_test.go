package commands_test

import (
	"log"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	})
})
