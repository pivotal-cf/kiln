package integration_test

import (
	"encoding/json"
	"github.com/Masterminds/semver"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("find-release-version", func() {
	var (
		tmpDir       string
		varsFilePath string

		varsFileContents   = os.Getenv("KILN_ACCEPTANCE_VARS_FILE_CONTENTS")
		someKilnfilePath                 = "./find-release-version-fixture/Kilnfile"
	)

	BeforeEach(func() {
		if varsFileContents == "" {
			Fail("please provide the KILN_ACCEPTANCE_VARS_FILE_CONTENTS environment variable")
		}

		var err error
		tmpDir, err = ioutil.TempDir("", "kiln-find-release-version")
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
		command := exec.Command(pathToMain, "find-release-version",
			"--kilnfile", someKilnfilePath,
			"--release", "uaa",
			"-vf", varsFilePath)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 60).Should(gexec.Exit(0))

		object := map[string]interface{}{}
		outputJson := strings.Split(string(session.Wait().Out.Contents()), "\n")
		err = json.Unmarshal([]byte(outputJson[1]), &object)
		Expect(err).NotTo(HaveOccurred())

		latestVersion, err := semver.NewVersion(object["version"].(string))
		oldVersion, err := semver.NewVersion("74.15.0")

		Expect(latestVersion.GreaterThan(oldVersion)).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
	})
})
