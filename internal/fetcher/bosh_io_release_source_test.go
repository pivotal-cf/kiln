package fetcher_test

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/extensions/table"

	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/fetcher"
	"github.com/pivotal-cf/kiln/pkg/release"
)

var _ = Describe("BOSHIOReleaseSource", func() {
	const (
		ID = fetcher.ReleaseSourceTypeBOSHIO
	)

	Describe("GetMatchedReleases from bosh.io", func() {
		Context("happy path", func() {
			var (
				releaseSource *fetcher.BOSHIOReleaseSource
				testServer    *ghttp.Server
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

				path, _ = regexp.Compile("/api/v1/releases/github.com/\\S+/metrics.*")
				testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `[{"version": "2.3.0"}]`))

				releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), logger)
			})

			AfterEach(func() {
				testServer.Close()
			})

			It("finds built releases which exist on bosh.io", func() {
				os := "ubuntu-xenial"
				version := "190.0.0"
				uaaRequirement := release.Requirement{Name: "uaa", Version: "73.3.0", StemcellOS: os, StemcellVersion: version}
				rabbitmqRequirement := release.Requirement{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: os, StemcellVersion: version}

				foundRelease, found, err := releaseSource.GetMatchedRelease(uaaRequirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				uaaURL := fmt.Sprintf("%s/d/github.com/cloudfoundry/uaa-release?v=73.3.0", testServer.URL())
				Expect(foundRelease).To(Equal(release.Remote{ID: release.ID{Name: "uaa", Version: "73.3.0"}, RemotePath: uaaURL, SourceID: fetcher.ReleaseSourceTypeBOSHIO}))

				foundRelease, found, err = releaseSource.GetMatchedRelease(rabbitmqRequirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				cfRabbitURL := fmt.Sprintf("%s/d/github.com/pivotal-cf/cf-rabbitmq-release?v=268.0.0", testServer.URL())
				Expect(foundRelease).To(Equal(release.Remote{ID: release.ID{Name: "cf-rabbitmq", Version: "268.0.0"}, RemotePath: cfRabbitURL, SourceID: fetcher.ReleaseSourceTypeBOSHIO}))

			})

		})

		When("a bosh release doesn't exist on bosh.io in any version", func() {
			var (
				testServer    *ghttp.Server
				releaseSource *fetcher.BOSHIOReleaseSource
			)

			BeforeEach(func() {
				logger := log.New(GinkgoWriter, "", 0)
				testServer = ghttp.NewServer()

				path, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/zzz.*")
				testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `null`))

				releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), logger)
			})

			AfterEach(func() {
				testServer.Close()
			})

			It("doesn't find releases which don't exist on bosh.io", func() {
				zzzRequirement := release.Requirement{Name: "zzz", Version: "999", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}
				_, found, err := releaseSource.GetMatchedRelease(zzzRequirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		When("a bosh release exists but the version does not", func() {
			var (
				testServer     *ghttp.Server
				releaseName    = "my-release"
				releaseVersion = "1.2.3"
				releaseSource  *fetcher.BOSHIOReleaseSource
			)

			BeforeEach(func() {
				testServer = ghttp.NewServer()

				pathRegex, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/.*")
				testServer.RouteToHandler("GET", pathRegex, ghttp.RespondWith(http.StatusOK, `[{"version": "4.0.4"}]`))

				releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), log.New(GinkgoWriter, "", 0))

			})

			AfterEach(func() {
				testServer.Close()
			})

			It("does not match that release", func() {
				_, found, err := releaseSource.GetMatchedRelease(release.Requirement{
					Name:            releaseName,
					Version:         releaseVersion,
					StemcellOS:      "ignored",
					StemcellVersion: "ignored",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
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

				releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), log.New(GinkgoWriter, "", 0))
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

					releaseID := release.ID{Name: releaseName, Version: releaseVersion}
					releaseRequirement := release.Requirement{
						Name:            releaseName,
						Version:         releaseVersion,
						StemcellOS:      "generic-os",
						StemcellVersion: "4.5.6",
					}

					foundRelease, found, err := releaseSource.GetMatchedRelease(releaseRequirement)

					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					expectedPath := fmt.Sprintf("%s/d/github.com/%s/%s%s?v=%s",
						testServer.URL(),
						organization,
						releaseName,
						suffix,
						releaseVersion,
					)

					Expect(foundRelease).To(Equal(release.Remote{ID: releaseID, RemotePath: expectedPath, SourceID: fetcher.ReleaseSourceTypeBOSHIO}))
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

	Describe("DownloadRelease", func() {
		const (
			release1Filename           = "some-1.2.3.tgz"
			release1ServerPath         = "/some-release"
			release1ServerFileContents = "totes-a-real-release"
		)
		var (
			releaseDir    string
			releaseSource *fetcher.BOSHIOReleaseSource
			testServer    *ghttp.Server

			release1ID release.ID
			release1   release.Remote

			release1Sha1 string
		)

		BeforeEach(func() {
			var err error
			releaseDir, err = ioutil.TempDir("", "kiln-releaseSource-test")
			Expect(err).NotTo(HaveOccurred())

			testServer = ghttp.NewServer()

			releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), log.New(GinkgoWriter, "", 0))

			release1ID = release.ID{Name: "some", Version: "1.2.3"}
			release1 = release.Remote{ID: release1ID, RemotePath: testServer.URL() + release1ServerPath, SourceID: fetcher.ReleaseSourceTypeBOSHIO}

			hash := sha1.New()
			_, err = io.Copy(hash, strings.NewReader(release1ServerFileContents))
			Expect(err).NotTo(HaveOccurred())

			release1Sha1 = hex.EncodeToString(hash.Sum(nil))

			testServer.RouteToHandler("GET", release1ServerPath,
				ghttp.RespondWith(http.StatusOK, release1ServerFileContents,
					nil,
				),
			)
		})

		AfterEach(func() {
			testServer.Close()
			_ = os.RemoveAll(releaseDir)
		})

		It("downloads the given releases into the release dir", func() {
			localRelease, err := releaseSource.DownloadRelease(releaseDir, release1, 1)

			Expect(err).NotTo(HaveOccurred())

			fullRelease1Path := filepath.Join(releaseDir, release1Filename)
			Expect(fullRelease1Path).To(BeAnExistingFile())

			release1DiskContents, err := ioutil.ReadFile(fullRelease1Path)
			Expect(err).NotTo(HaveOccurred())
			Expect(release1DiskContents).To(BeEquivalentTo(release1ServerFileContents))

			Expect(localRelease).To(Equal(release.Local{ID: release1ID, LocalPath: fullRelease1Path, SHA1: release1Sha1}))
		})
	})

	Describe("FindReleaseVersion from bosh.io", func() {
		var (
			releaseSource *fetcher.BOSHIOReleaseSource
			testServer    *ghttp.Server
		)
		When("a bosh release exist on bosh.io", func() {
			BeforeEach(func() {
				logger := log.New(GinkgoWriter, "", 0)
				testServer = ghttp.NewServer()

				path, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/cf-rabbitmq.*")
				testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `[{"name":"github.com/cloudfoundry/cf-rabbitmq-release","version":"309.0.5","url":"https://bosh.io/d/github.com/cloudfoundry/cf-rabbitmq-release?v=309.0.0","sha1":"5df538657c2cc830bda679420a9b162682018ded"},{"name":"github.com/cloudfoundry/cf-rabbitmq-release","version":"308.0.0","url":"https://bosh.io/d/github.com/cloudfoundry/cf-rabbitmq-release?v=308.0.0","sha1":"56202c9a466a8394683ae432ee2dea21ef6ef865"}]`))

				releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), logger)
			})

			AfterEach(func() {
				testServer.Close()
			})
			When("there is no version requirement", func() {
				It("gets the latest version from bosh.io", func() {
					rabbitmqRequirement := release.Requirement{Name: "cf-rabbitmq"}

					foundRelease, found, err := releaseSource.FindReleaseVersion(rabbitmqRequirement)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					cfRabbitURL := fmt.Sprintf("%s/d/github.com/cloudfoundry/cf-rabbitmq-release?v=309.0.5", testServer.URL())
					Expect(foundRelease).To(Equal(release.Remote{ID: release.ID{Name: "cf-rabbitmq", Version: "309.0.5"}, SHA: "5df538657c2cc830bda679420a9b162682018ded", RemotePath: cfRabbitURL, SourceID: fetcher.ReleaseSourceTypeBOSHIO}))

				})
			})
			When("there is a version requirement", func() {
				It("gets the latest version from bosh.io", func() {
					rabbitmqRequirement := release.Requirement{Name: "cf-rabbitmq", VersionConstraint: "~309"}

					foundRelease, found, err := releaseSource.FindReleaseVersion(rabbitmqRequirement)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					cfRabbitURL := fmt.Sprintf("%s/d/github.com/cloudfoundry/cf-rabbitmq-release?v=309.0.5", testServer.URL())
					Expect(foundRelease).To(Equal(release.Remote{ID: release.ID{Name: "cf-rabbitmq", Version: "309.0.5"},
						SHA:        "5df538657c2cc830bda679420a9b162682018ded",
						RemotePath: cfRabbitURL,
						SourceID:   fetcher.ReleaseSourceTypeBOSHIO}))
				})
			})
		})
		When("a bosh release does not exist on bosh.io", func() {
			BeforeEach(func() {
				logger := log.New(GinkgoWriter, "", 0)
				testServer = ghttp.NewServer()

				path, _ := regexp.Compile("/api/v1/releases/github.com/\\S+/cf-rabbitmq.*")
				testServer.RouteToHandler("GET", path, ghttp.RespondWith(http.StatusOK, `null`))

				releaseSource = fetcher.NewBOSHIOReleaseSource(ID, false, testServer.URL(), logger)
			})

			AfterEach(func() {
				testServer.Close()
			})

			It("returns not found", func() {
				rabbitmqRequirement := release.Requirement{Name: "cf-rabbitmq"}

				foundRelease, found, err := releaseSource.FindReleaseVersion(rabbitmqRequirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundRelease).To(Equal(release.Remote{}))
			})
		})
	})
})
