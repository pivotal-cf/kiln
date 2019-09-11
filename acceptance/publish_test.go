package acceptance_test

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/go-pivnet"
	"github.com/pivotal-cf/go-pivnet/logshim"
)

var _ = Describe("publish", func() {
	const (
		slug             = "elastic-runtime"
		publishDateBeta  = "2019-10-28"
		publishDateRC    = "2019-11-11"
		publishDateGA    = "2019-12-06"

		// we are testing on a specific release on pivnet
		releaseID = 384471

		kilnfileBody = `---
slug: ` + slug + `
publish_dates:
  beta: ` + publishDateBeta + `
  rc: ` + publishDateRC + `
  ga: ` + publishDateGA
	)

	var (
		client pivnet.Client
		token, tmpDir, initialDir, host,
		releaseVersion string
	)

	restoreState := func() {
		token = os.Getenv("PIVNET_TOKEN")
		if token == "" {
			Skip("please provide the PIVNET_TOKEN environment variable")
		}
		host = "https://pivnet-integration.cfapps.io"

		stdoutLogger := log.New(os.Stdout, "", log.LstdFlags)
		stderrLogger := log.New(os.Stderr, "", log.LstdFlags)
		logger := logshim.NewLogShim(stdoutLogger, stderrLogger, false)
		config := pivnet.ClientConfig{Host: host}
		accessTokenService := pivnet.NewAccessTokenOrLegacyToken(token, config.Host)
		client = pivnet.NewClient(accessTokenService, config, logger)

		releaseVersion = "2.2.17-build.6"
		rel, err := client.Releases.Get(slug, releaseID)
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
		if rel.Version == releaseVersion && rel.ReleaseType == "Developer Release" {
			return
		}
		rel.Version = releaseVersion
		rel.ReleaseType = "Maintenance Release"
		_, err = client.Releases.Update(slug, rel)
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
	}

	BeforeEach(func() {
		restoreState()
		var err error
		tmpDir, err = ioutil.TempDir("", "kiln-publish-test")
		Expect(err).NotTo(HaveOccurred())
		initialDir, _ = os.Getwd()
		os.Chdir(tmpDir)
		ioutil.WriteFile("version", []byte(releaseVersion), 0777)
		ioutil.WriteFile("Kilnfile", []byte(kilnfileBody), 0777)
	})

	AfterEach(func() {
		os.Chdir(initialDir)
		os.RemoveAll(tmpDir)
		restoreState()
	})

	It("updates release on pivnet", func() {
		command := exec.Command(pathToMain, "publish",
			"--pivnet-token", token,
			"--pivnet-host", host)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 20).Should(gexec.Exit(0))

		rel, err := client.Releases.Get(slug, releaseID)
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(rel.Version).To(Equal("2.2.17"))
		Expect(string(rel.ReleaseType)).To(Equal("Alpha Release"))
	})
})
