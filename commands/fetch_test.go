package commands_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/commands"
)

var _ = Describe("Fetch", func() {
	var (
		tmpDir                string
		someReleasesDirectory string

		fetch commands.Fetch
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "command-test")
		Expect(err).NotTo(HaveOccurred())

		someReleasesDirectory, err = ioutil.TempDir(tmpDir, "releases")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Execute", func() {

		It("fetches releases", func() {
			err := fetch.Execute([]string{
				"--assets-file", "fixtures/assets.yml",
				"--releases-directory", someReleasesDirectory,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Usage", func() {
		It("returns usage information for the command", func() {
			Expect(fetch.Usage()).To(Equal(jhanda.Usage{
				Description:      "Fetches releases listed in assets file from S3 and downloads it locally",
				ShortDescription: "fetches releases",
				Flags:            fetch,
			}))
		})
	})
})
