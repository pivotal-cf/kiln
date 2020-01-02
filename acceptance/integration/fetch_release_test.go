package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("fetch", func() {
	var (
		tmpDir string

		productDirectory   = os.Getenv("PRODUCT_DIRECTORY")
		awsAccessKeyID     = os.Getenv("AWS_ACCESS_KEY_ID")
		awsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	)

	BeforeEach(func() {
		Expect(productDirectory).NotTo(BeEmpty())
		Expect(awsAccessKeyID).NotTo(BeEmpty())
		Expect(awsSecretAccessKey).NotTo(BeEmpty())

		var err error
		tmpDir, err = ioutil.TempDir("", "kiln-fetch")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("fetches releases", func() {
		command := exec.Command(pathToMain, "fetch",
			"--kilnfile", filepath.Join(productDirectory, "Kilnfile"),
			"--releases-directory", tmpDir,
			"--download-threads", "8",
			"--variable", "aws_access_key_id="+awsAccessKeyID,
			"--variable", "aws_secret_access_key="+awsSecretAccessKey,
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 300).Should(gexec.Exit(0))
	})
})
