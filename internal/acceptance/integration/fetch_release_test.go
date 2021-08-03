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
		tmpDir       string
		varsFilePath string

		productDirectory = os.Getenv("PRODUCT_DIRECTORY")
		varsFileContents = os.Getenv("KILN_ACCEPTANCE_VARS_FILE_CONTENTS")
	)

	BeforeEach(func() {
		if varsFileContents == "" {
			Fail("please provide the KILN_ACCEPTANCE_VARS_FILE_CONTENTS environment variable")
		}

		if productDirectory == "" {
			Fail("please provide the PRODUCT_DIRECTORY environment variable (usually the path to p-isolation-segment)")
		}

		var err error
		tmpDir, err = ioutil.TempDir("", "kiln-fetch")
		Expect(err).NotTo(HaveOccurred())

		varsFilePath = filepath.Join(tmpDir, "variables.yml")
		Expect(
			ioutil.WriteFile(varsFilePath, []byte(varsFileContents), 0600),
		).To(Succeed())
	})

	AfterEach(func() {
		Expect(
			os.RemoveAll(tmpDir),
		).To(Succeed())
	})

	It("fetches releases", func() {
		command := exec.Command(pathToMain, "fetch",
			"--kilnfile", filepath.Join(productDirectory, "Kilnfile"),
			"--releases-directory", tmpDir,
			"--download-threads", "8",
			"--variables-file", varsFilePath,
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 300).Should(gexec.Exit(0))
	})
})
