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
	})
})
