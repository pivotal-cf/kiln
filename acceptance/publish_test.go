package acceptance_test

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/go-pivnet"
	"github.com/pivotal-cf/go-pivnet/logshim"
)

var _ = Describe("publish", func() {
	const (
		slug           = "elastic-runtime"
		releaseVersion = "2.2.17-build.6"

		// we are testing on a specific release on pivnet
		releaseID = 384471
	)

	var (
		client       pivnet.Client
		token, tmpDir, initialDir, host,
		kilnfileBody string
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

		rel, err := client.Releases.Get(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())

		if rel.Version != releaseVersion || rel.ReleaseType != "Developer Release" {
			rel.Version = releaseVersion
			rel.ReleaseType = "Developer Release"
			_, err = client.Releases.Update(slug, rel)
			Expect(err).NotTo(HaveOccurred())
		}

		releaseFiles, err := client.ProductFiles.ListForRelease(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())
		for _, file := range releaseFiles {
			if file.FileType == "Open Source License" {
				err = client.ProductFiles.RemoveFromRelease(slug, releaseID, file.ID)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}

	BeforeEach(func() {
		restoreState()
		var err error

		now := time.Now().UTC()
		days := 24 * time.Hour
		publishDateGA := now.Add(-1 * days)
		publishDateRC := publishDateGA.Add(-14 * days)
		publishDateBeta := publishDateRC.Add(-13 * days)

		dateFormat := "2006-01-02"

		kilnfileBody = `---
slug: ` + slug + `
publish_dates:
  beta: ` + publishDateBeta.Format(dateFormat) + `
  rc: ` + publishDateRC.Format(dateFormat) + `
  ga: ` + publishDateGA.Format(dateFormat)

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
		Expect(err).NotTo(HaveOccurred())
		Expect(rel.Version).To(Equal("2.2.17"))
		Expect(string(rel.ReleaseType)).To(Equal("Maintenance Release"))

		releaseFiles, err := client.ProductFiles.ListForRelease(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())

		var licenseFile pivnet.ProductFile
		for _, file := range releaseFiles {
			if file.FileType == "Open Source License" {
				licenseFile = file
				break
			}
		}
		Expect(licenseFile).NotTo(Equal(pivnet.ProductFile{}))
		Expect(licenseFile.Name).To(Equal("PCF Pivotal Application Service v2.2 OSL"))
	})
})
