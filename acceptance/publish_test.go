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
	"github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/go-pivnet/v2/logshim"
)

var _ = Describe("publish", func() {
	const (
		slug           = "elastic-runtime"
		releaseVersion = "2.2.17-build.6"

		// we are testing on a specific release on pivnet
		releaseID = 384471
	)

	var (
		client pivnet.Client
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
		accessTokenService := pivnet.NewAccessTokenOrLegacyToken(token, config.Host, false)
		client = pivnet.NewClient(accessTokenService, config, logger)

		rel, err := client.Releases.Get(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())

		if rel.Version != releaseVersion || rel.ReleaseType != "Developer Release" {
			rel.Version = releaseVersion
			rel.ReleaseDate = ""
			rel.EndOfSupportDate = ""
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

		kilnfileBody = `---
slug: ` + slug

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
			"--window", "ga",
			"--pivnet-token", token,
			"--pivnet-host", host)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 20).Should(gexec.Exit(0))

		rel, err := client.Releases.Get(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(rel.Version).To(Equal("2.2.17"))
		Expect(rel.ReleaseType).To(BeEquivalentTo("Maintenance Release"))
		dateFormat := "2006-01-02"
		Expect(rel.ReleaseDate).To(Equal(time.Now().Format(dateFormat)))
		Expect(rel.EndOfSupportDate).To(Equal("2019-04-30")) // EOGS for 2.2

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
