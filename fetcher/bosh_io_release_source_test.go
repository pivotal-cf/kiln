package fetcher_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pivotal-cf/kiln/internal/cargo"

	. "github.com/onsi/ginkgo/extensions/table"

	"github.com/onsi/gomega/ghttp"

	"github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMatchedReleases from bosh.io", func() {
	var ignoredStemcell = cargo.Stemcell{OS: "ignored", Version: "ignored"}

	Context("happy path", func() {
		var (
			releaseSource     *fetcher.BOSHIOReleaseSource
			desiredReleaseSet fetcher.ReleaseSet
			testServer        *ghttp.Server
		)

		BeforeEach(func() {
			logger := log.New(GinkgoWriter, "", 0)
			testServer = ghttp.NewServer()

			path, _ := regexp.Compile("/api/v1/releases/github.com/pivotal-cf/cf-rabbitmq.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `[{"version": "268.0.0"}]`))

			path, _ = regexp.Compile("/api/v1/releases/github.com/\\S+/cf-rabbitmq.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `null`))

			path, _ = regexp.Compile("/api/v1/releases/github.com/\\S+/uaa.*")
			testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `[{"version": "73.3.0"}]`))

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
			desiredReleaseSet = fetcher.ReleaseSet{
				fetcher.ReleaseID{Name: "uaa", Version: "73.3.0"}:          fetcher.CompiledRelease{ID: fetcher.ReleaseID{Name: "uaa", Version: "73.3.0"}, StemcellOS: os, StemcellVersion: version},
				fetcher.ReleaseID{Name: "zzz", Version: "999"}:             fetcher.CompiledRelease{ID: fetcher.ReleaseID{Name: "zzz", Version: "999"}, StemcellOS: os, StemcellVersion: version},
				fetcher.ReleaseID{Name: "cf-rabbitmq", Version: "268.0.0"}: fetcher.CompiledRelease{ID: fetcher.ReleaseID{Name: "cf-rabbitmq", Version: "268.0.0"}, StemcellOS: os, StemcellVersion: version},
			}

			foundReleases, err := releaseSource.GetMatchedReleases(desiredReleaseSet, ignoredStemcell)
			uaaURL := fmt.Sprintf("%s/d/github.com/cloudfoundry/uaa-release?v=73.3.0", testServer.URL())
			cfRabbitURL := fmt.Sprintf("%s/d/github.com/pivotal-cf/cf-rabbitmq-release?v=268.0.0", testServer.URL())

			Expect(err).ToNot(HaveOccurred())
			Expect(foundReleases).To(HaveLen(2))
			Expect(foundReleases).To(HaveKeyWithValue(fetcher.ReleaseID{Name: "uaa", Version: "73.3.0"},
				fetcher.BuiltRelease{ID: fetcher.ReleaseID{Name: "uaa", Version: "73.3.0"}, Path: uaaURL}))
			Expect(foundReleases).To(HaveKeyWithValue(fetcher.ReleaseID{Name: "cf-rabbitmq", Version: "268.0.0"},
				fetcher.BuiltRelease{ID: fetcher.ReleaseID{Name: "cf-rabbitmq", Version: "268.0.0"}, Path: cfRabbitURL}))
		})
	})

	When("a bosh release exists but the version does not", func() {
		var (
			testServer     *ghttp.Server
			releaseName    = "my-release"
			releaseVersion = "1.2.3"
			releaseSource  *fetcher.BOSHIOReleaseSource

			foundReleases         fetcher.ReleaseSet
			getMatchedReleasesErr error
		)

		BeforeEach(func() {
			testServer = ghttp.NewServer()

			pathRegex, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/.*")
			testServer.RouteToHandler("GET", pathRegex, ghttp.RespondWith(http.StatusOK, `[{"version": "4.0.4"}]`))

			releaseSource = fetcher.NewBOSHIOReleaseSource(
				log.New(GinkgoWriter, "", 0),
				testServer.URL(),
			)

		})

		AfterEach(func() {
			testServer.Close()
		})

		JustBeforeEach(func() {
			releaseID := fetcher.ReleaseID{Name: releaseName, Version: releaseVersion}

			foundReleases, getMatchedReleasesErr = releaseSource.GetMatchedReleases(
				fetcher.ReleaseSet{releaseID: fetcher.CompiledRelease{}},
				ignoredStemcell,
			)
		})

		It("does not match that release", func() {
			Expect(getMatchedReleasesErr).NotTo(HaveOccurred())
			Expect(foundReleases).To(HaveLen(0))
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
				testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, fmt.Sprintf(`[{"version": %q}]`, releaseVersion)))

				pathRegex, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/.*")
				testServer.RouteToHandler("GET", pathRegex, ghttp.RespondWith(http.StatusOK, `null`))

				releaseID := fetcher.ReleaseID{Name: releaseName, Version: releaseVersion}
				compiledRelease := fetcher.CompiledRelease{
					ID:              releaseID,
					StemcellOS:      "generic-os",
					StemcellVersion: "4.5.6",
					Path:            "",
				}

				foundReleases, err := releaseSource.GetMatchedReleases(
					fetcher.ReleaseSet{releaseID: compiledRelease},
					ignoredStemcell,
				)

				Expect(err).NotTo(HaveOccurred())
				expectedPath := fmt.Sprintf("%s/d/github.com/%s/%s%s?v=%s",
					testServer.URL(),
					organization,
					releaseName,
					suffix,
					releaseVersion,
				)

				expectedRelease := fetcher.BuiltRelease{
					ID:   releaseID,
					Path: expectedPath,
				}

				Expect(foundReleases).To(HaveKeyWithValue(releaseID, expectedRelease))
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

		release1ID                 fetcher.ReleaseID
		release1                   fetcher.BuiltRelease
		release1ServerPath         string
		release1Filename           string
		release1ServerFileContents string

		release2ID                 fetcher.ReleaseID
		release2                   fetcher.BuiltRelease
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

		release1ID = fetcher.ReleaseID{Name: "some", Version: "1.2.3"}
		release1ServerPath = "/some-release"
		release1 = fetcher.BuiltRelease{ID: release1ID, Path: testServer.URL() + release1ServerPath}
		release1Filename = "some-1.2.3.tgz"
		release1ServerFileContents = "totes-a-real-release"

		release2ID = fetcher.ReleaseID{Name: "another", Version: "2.3.4"}
		release2ServerPath = "/releases/another/release/2.3.4"
		release2 = fetcher.BuiltRelease{ID: release2ID, Path: testServer.URL() + release2ServerPath}
		release2Filename = "another-2.3.4.tgz"
		release2ServerFileContents = "blah-blah-blah deploy instructions blah blah"

		testServer.RouteToHandler("GET", release1ServerPath,
			ghttp.RespondWith(http.StatusOK, release1ServerFileContents,
				nil,
			),
		)
		testServer.RouteToHandler("GET", release2ServerPath,
			ghttp.RespondWith(http.StatusOK, release2ServerFileContents,
				nil,
			),
		)
	})

	AfterEach(func() {
		testServer.Close()
		_ = os.RemoveAll(releaseDir)
	})

	It("downloads the given releases into the release dir", func() {
		matchedReleases := fetcher.ReleaseSet{
			release1ID: release1,
			release2ID: release2,
		}
		err := releaseSource.DownloadReleases(releaseDir,
			matchedReleases,
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
