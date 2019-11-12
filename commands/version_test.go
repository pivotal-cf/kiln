package commands_test

import (
	"log"
	"strings"

	"github.com/pivotal-cf/jhanda"
	. "github.com/pivotal-cf/kiln/commands"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	var (
		writer  strings.Builder
		logger  *log.Logger
		version Version
	)

	BeforeEach(func() {
		logger = log.New(&writer, "", 0)
		version = NewVersion(logger, "1.2.3-build.4")
	})

	Describe("Execute", func() {
		It("prints the version number", func() {
			err := version.Execute(nil)
			Expect(err).NotTo(HaveOccurred())

			Expect((&writer).String()).To(Equal("kiln version 1.2.3-build.4\n"))
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			version := NewVersion(nil, "")
			Expect(version.Usage()).To(Equal(jhanda.Usage{
				Description:      "This command prints the kiln release version number.",
				ShortDescription: "prints the kiln release version",
			}))
		})
	})
})
