package fetcher_test

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/onsi/gomega/ghttp"

	"github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("GetMatchedReleases from bosh.io", func() {
	var (
		releaseSource             *fetcher.BOSHIOReleaseSource
		desiredCompiledReleaseSet cargo.CompiledReleaseSet
		testServer                *ghttp.Server
	)

	BeforeEach(func() {
		logger := log.New(nil, "", 0)
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

	It("returns releases which exists on bosh.io", func() {
		os := "ubuntu-xenial"
		version := "190.0.0"
		desiredCompiledReleaseSet = cargo.CompiledReleaseSet{
			{Name: "uaa", Version: "73.3.0", StemcellOS: os, StemcellVersion: version}:          "",
			{Name: "zzz", Version: "999", StemcellOS: os, StemcellVersion: version}:             "",
			{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: os, StemcellVersion: version}: "",
		}

		foundReleases, err := releaseSource.GetMatchedReleases(desiredCompiledReleaseSet)
		uaaURL := fmt.Sprintf("%s/d/github.com/cloudfoundry/uaa-release?v=73.3.0", testServer.URL())
		cfRabbitURL := fmt.Sprintf("%s/d/github.com/pivotal-cf/cf-rabbitmq-release?v=268.0.0", testServer.URL())

		Expect(err).ToNot(HaveOccurred())
		Expect(foundReleases).To(HaveLen(2))
		Expect(foundReleases).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "uaa", Version: "73.3.0", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, uaaURL))
		Expect(foundReleases).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, cfRabbitURL))
	})
})
