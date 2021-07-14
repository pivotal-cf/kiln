package integration_test

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/gomega/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/gstruct"
	pivnet "github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/go-pivnet/v2/logshim"
)

var _ = Describe("publish", func() {
	const (
		slug           = "elastic-runtime"
		releaseVersion = "2.7.0-build.247"

		// we are testing on a specific release on pivnet
		releaseID      = 471301
		preGaUserGroup = "Dell/EMC Early Access Group"
	)

	var (
		client pivnet.Client
		token, tmpDir, initialDir, host,
		kilnfileBody string
	)

	restoreState := func() {
		token = os.Getenv("PIVNET_TOKEN")
		if token == "" {
			Fail("please provide the PIVNET_TOKEN environment variable")
		}
		host = "https://network-integration.tanzu.vmware.com/"

		stdoutLogger := log.New(os.Stdout, "", log.LstdFlags)
		stderrLogger := log.New(os.Stderr, "", log.LstdFlags)
		logger := logshim.NewLogShim(stdoutLogger, stderrLogger, false)
		config := pivnet.ClientConfig{
			Host:              host,
			SkipSSLValidation: true,
		}
		accessTokenService := pivnet.NewAccessTokenOrLegacyToken(token, config.Host, false)
		client = pivnet.NewClient(accessTokenService, config, logger)

		rel, err := client.Releases.Get(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())

		if rel.Version != releaseVersion || rel.ReleaseType != "Developer Release" {
			rel.Version = releaseVersion
			rel.ReleaseDate = ""
			rel.ReleaseType = "Developer Release"
			rel.EndOfSupportDate = ""
			rel.Availability = "Selected User Groups Only"
			rel.EULA.Slug = "vmware-prerelease-eula"
			_, err = client.Releases.Update(slug, rel)
			Expect(err).NotTo(HaveOccurred())
		}

		userGroups, err := client.UserGroups.ListForRelease(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())
		for _, userGroup := range userGroups {
			if userGroup.Name == preGaUserGroup {
				err = client.UserGroups.RemoveFromRelease(slug, releaseID, userGroup.ID)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}

	BeforeEach(func() {
		restoreState()
		var err error

		kilnfileBody = `---
slug: ` + slug + `
pre_ga_user_groups:
  - ` + preGaUserGroup

		tmpDir, err = ioutil.TempDir("", "kiln-publish-test")
		Expect(err).NotTo(HaveOccurred())
		initialDir, _ = os.Getwd()
		err = os.Chdir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		_ = ioutil.WriteFile("version", []byte(releaseVersion), 0777)
		_ = ioutil.WriteFile("Kilnfile", []byte(kilnfileBody), 0777)
	})

	AfterEach(func() {
		err := os.Chdir(initialDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.RemoveAll(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		restoreState()
	})

	It("updates release on pivnet", func() {
		command := exec.Command(pathToMain, "publish",
			"--window", "rc",
			"--pivnet-token", token,
			"--pivnet-host", host)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, 60).Should(gexec.Exit(0))

		rel, err := client.Releases.Get(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(rel.Version).To(Equal("2.7.0-rc.4"))
		Expect(rel.ReleaseType).To(BeEquivalentTo("Release Candidate"))
		dateFormat := "2006-01-02"
		Expect(rel.ReleaseDate).To(Equal(time.Now().Format(dateFormat)))
		Expect(rel.Availability).To(Equal("Selected User Groups Only"))

		userGroups, err := client.UserGroups.ListForRelease(slug, releaseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(userGroups).To(ContainElement(HaveFieldWithValue("Name", preGaUserGroup)))
	})
})

func HaveFieldWithValue(field string, value interface{}) types.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{field: Equal(value)})
}
