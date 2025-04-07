package pivnet_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/internal/pivnet"
)

var _ = Describe("PivNet (network.pivotal.io)", func() {
	When("making an http request to pivotal network", func() {
		var (
			pivnetService pivnet.Service
			serverMock    *fakes.RoundTripper
			simpleRequest *http.Request
			requestErr    error
		)

		BeforeEach(func() {
			pivnetService = pivnet.Service{}
			simpleRequest, _ = http.NewRequest(http.MethodGet, "/", nil)

			serverMock = &fakes.RoundTripper{}
			serverMock.Results.Res = &http.Response{}
			pivnetService.Client = &http.Client{
				Transport: serverMock,
			}
		})

		JustBeforeEach(func() {
			_, requestErr = pivnetService.Do(simpleRequest)
			Expect(requestErr).NotTo(HaveOccurred())
		})

		When("an zero-value client is used", func() {
			It("makes a request with resonable defaults", func() {
				Expect(serverMock.Params.Req.Header.Get("Accept")).To(Equal("application/json"))
				Expect(serverMock.Params.Req.Header.Get("Content-Type")).To(Equal("application/json"))
				Expect(serverMock.Params.Req.Header.Get("User-Agent")).To(Equal("kiln"))

				Expect(serverMock.Params.Req.Header.Get("Authorization")).To(BeEmpty())
			})
		})

		When("a UAA token is set", func() {
			BeforeEach(func() {
				pivnetService.UAAAPIToken = "some-token"
			})

			It("makes a request with correct auth headers", func() {
				Expect(serverMock.Params.Req.Header.Get("Authorization")).To(Equal("Bearer some-token"))
			})
		})

		When("to fetch releases", func() {
			BeforeEach(func() {
				pivnetService = pivnet.Service{}
				pivnetService.UAAAPIToken = "some-token"

				serverMock = &fakes.RoundTripper{}
				serverMock.Results.Res = &http.Response{
					StatusCode: http.StatusOK,
					Body:       fakes.NewReadCloser(`{"releases":[{"version": "456.98"},{"version": "456.99"},{"version": "456.100"}]}`),
				}
				pivnetService.Client = &http.Client{
					Transport: serverMock,
				}

			})

			It("returns all releases for the given product slug", func() {
				Expect(serverMock.Params.Req.Header.Get("Authorization")).To(Equal("Bearer some-token"))
				releases, err := pivnetService.Releases("some-product")
				Expect(len(releases)).To(Equal(3))
				Expect(releases[0].Version).To(Equal("456.98"))
				Expect(releases[1].Version).To(Equal("456.99"))
				Expect(releases[2].Version).To(Equal("456.100"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
