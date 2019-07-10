package fetcher_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/extensions/table"

	"github.com/onsi/gomega/ghttp"

	"github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMatchedReleases from bosh.io", func() {
	Context("happy path", func() {
		var (
			releaseSource             *fetcher.BOSHIOReleaseSource
			desiredCompiledReleaseSet fetcher.CompiledReleaseSet
			testServer                *ghttp.Server
		)

		BeforeEach(func() {
			logger := log.New(GinkgoWriter, "", 0)
			testServer = ghttp.NewServer()

			path, _ := regexp.Compile("/api/v1/releases/github.com/pivotal-cf/cf-rabbitmq.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, ``))

			path, _ = regexp.Compile("/api/v1/releases/github.com/\\S+/cf-rabbitmq.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `null`))

			path, _ = regexp.Compile("/api/v1/releases/github.com/\\S+/uaa.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, ``))

			path, _ = regexp.Compile("/api/v1/releases/github.com/\\S+/zzz.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `null`))

			releaseSource = fetcher.NewBOSHIOReleaseSource(
				logger,
				testServer.URL(),
			)
		})

		AfterEach(func() {
			testServer.Close()
		})

		It("returns built releases which exist on bosh.io", func() {
			os := "ubuntu-xenial"
			version := "190.0.0"
			desiredCompiledReleaseSet = fetcher.CompiledReleaseSet{
				{Name: "uaa", Version: "73.3.0", StemcellOS: os, StemcellVersion: version}:          "",
				{Name: "zzz", Version: "999", StemcellOS: os, StemcellVersion: version}:             "",
				{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: os, StemcellVersion: version}: "",
			}

			foundReleases, err := releaseSource.GetMatchedReleases(desiredCompiledReleaseSet)
			uaaURL := fmt.Sprintf("%s/d/github.com/cloudfoundry/uaa-release?v=73.3.0", testServer.URL())
			cfRabbitURL := fmt.Sprintf("%s/d/github.com/pivotal-cf/cf-rabbitmq-release?v=268.0.0", testServer.URL())

			Expect(err).ToNot(HaveOccurred())
			Expect(foundReleases).To(HaveLen(2))
			Expect(foundReleases).To(HaveKeyWithValue(fetcher.CompiledRelease{Name: "uaa", Version: "73.3.0", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, uaaURL))
			Expect(foundReleases).To(HaveKeyWithValue(fetcher.CompiledRelease{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, cfRabbitURL))
		})
	})

	Describe("releases can exist in many orgs with various suffixes", func() {
		var (
			testServer     *ghttp.Server
			releaseName    = "my-release"
			releaseVersion = "1.2.3"
			releaseSource  *fetcher.BOSHIOReleaseSource
		)

		BeforeEach(func() {
			testServer = ghttp.NewServer()

			releaseSource = fetcher.NewBOSHIOReleaseSource(
				log.New(GinkgoWriter, "", 0),
				testServer.URL(),
			)
		})

		AfterEach(func() {
			testServer.Close()
		})

		DescribeTable("searching multiple paths for each release",
			func(organization, suffix string) {
				path := fmt.Sprintf("/api/v1/releases/github.com/%s/%s%s", organization, releaseName, suffix)
				testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, ``))

				pathRegex, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/.*")
				testServer.RouteToHandler("GET", pathRegex, ghttp.RespondWith(http.StatusOK, `null`))

				compiledRelease := fetcher.CompiledRelease{
					Name:            releaseName,
					Version:         releaseVersion,
					StemcellOS:      "generic-os",
					StemcellVersion: "4.5.6",
				}

				foundReleases, err := releaseSource.GetMatchedReleases(fetcher.CompiledReleaseSet{compiledRelease: ""})

				Expect(err).NotTo(HaveOccurred())
				expectedPath := fmt.Sprintf("%s/d/github.com/%s/%s%s?v=%s",
					testServer.URL(),
					organization,
					releaseName,
					suffix,
					releaseVersion,
				)
				Expect(foundReleases).To(HaveKeyWithValue(compiledRelease, expectedPath))
			},

			Entry("cloudfoundry org, no suffix", "cloudfoundry", ""),
			Entry("cloudfoundry org, -release suffix", "cloudfoundry", "-release"),
			Entry("cloudfoundry org, -bosh-release suffix", "cloudfoundry", "-bosh-release"),
			Entry("cloudfoundry org, -boshrelease suffix", "cloudfoundry", "-boshrelease"),
			Entry("pivotal-cf org, no suffix", "pivotal-cf", ""),
			Entry("pivotal-cf org, -release suffix", "pivotal-cf", "-release"),
			Entry("pivotal-cf org, -bosh-release suffix", "pivotal-cf", "-bosh-release"),
			Entry("pivotal-cf org, -boshrelease suffix", "pivotal-cf", "-boshrelease"),
			Entry("frodenas org, no suffix", "frodenas", ""),
			Entry("frodenas org, -release suffix", "frodenas", "-release"),
			Entry("frodenas org, -bosh-release suffix", "frodenas", "-bosh-release"),
			Entry("frodenas org, -boshrelease suffix", "frodenas", "-boshrelease"),
		)
	})
})

var _ = Describe("DownloadReleases", func() {
	var (
		releaseDir    string
		releaseSource *fetcher.BOSHIOReleaseSource
		testServer    *ghttp.Server

		release1                   fetcher.CompiledRelease
		release1ServerPath         string
		release1Filename           string
		release1ServerFileContents string

		release2                   fetcher.CompiledRelease
		release2ServerPath         string
		release2Filename           string
		release2ServerFileContents string
	)

	BeforeEach(func() {
		var err error
		releaseDir, err = ioutil.TempDir("", "kiln-releaseSource-test")
		Expect(err).NotTo(HaveOccurred())

		testServer = ghttp.NewServer()

		releaseSource = fetcher.NewBOSHIOReleaseSource(
			log.New(GinkgoWriter, "", 0),
			testServer.URL(),
		)

		release1 = fetcher.CompiledRelease{Name: "some-release", Version: "1.2.3"}
		release1ServerPath = "/some-release"
		release1Filename = "some-release.tgz"
		release1ServerFileContents = "totes-a-real-release"

		release2 = fetcher.CompiledRelease{Name: "another-release", Version: "2.3.4"}
		release2ServerPath = "/releases/another/release/2.3.4"
		release2Filename = "release-2.3.4.tgz"
		release2ServerFileContents = "blah-blah-blah deploy instructions blah blah"

		testServer.RouteToHandler("GET", release1ServerPath,
			ghttp.RespondWith(http.StatusOK, release1ServerFileContents,
				http.Header{"Content-Disposition": []string{"attachment; filename=" + release1Filename}},
			),
		)
		testServer.RouteToHandler("GET", release2ServerPath,
			ghttp.RespondWith(http.StatusOK, release2ServerFileContents,
				http.Header{"Content-Disposition": []string{"attachment; filename=" + release2Filename}},
			),
		)
	})

	AfterEach(func() {
		testServer.Close()
		_ = os.RemoveAll(releaseDir)
	})

	It("downloads the given releases into the release dir", func() {
		err := releaseSource.DownloadReleases(releaseDir,
			fetcher.CompiledReleaseSet{
				release1: testServer.URL() + release1ServerPath,
				release2: testServer.URL() + release2ServerPath,
			},
			1,
		)

		Expect(err).NotTo(HaveOccurred())

		fullRelease1Path := filepath.Join(releaseDir, release1Filename)
		fullRelease2Path := filepath.Join(releaseDir, release2Filename)
		Expect(fullRelease1Path).To(BeAnExistingFile())
		Expect(fullRelease2Path).To(BeAnExistingFile())

		release1DiskContents, err := ioutil.ReadFile(fullRelease1Path)
		Expect(err).NotTo(HaveOccurred())
		Expect(release1DiskContents).To(BeEquivalentTo(release1ServerFileContents))

		release2DiskContents, err := ioutil.ReadFile(fullRelease2Path)
		Expect(err).NotTo(HaveOccurred())
		Expect(release2DiskContents).To(BeEquivalentTo(release2ServerFileContents))
	})
})
