package fetcher_test

import (
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/fetcher/fakes"
)

var _ = Describe("PivNet (network.pivotal.io)", func() {
	When("making an http request to pivotal network", func() {
		var (
			pivnet        fetcher.Pivnet
			serverMock    *fakes.RoundTripper
			simpleRequest *http.Request
			requestErr    error
		)

		BeforeEach(func() {
			pivnet = fetcher.Pivnet{}
			simpleRequest, _ = http.NewRequest(http.MethodGet, "/", nil)

			serverMock = &fakes.RoundTripper{}
			serverMock.Results.Res = &http.Response{}
			http.DefaultClient.Transport = serverMock
		})

		JustBeforeEach(func() {
			_, requestErr = pivnet.Do(simpleRequest)
			Expect(requestErr).NotTo(HaveOccurred())
		})

		When("an a zero-value client is used", func() {
			It("makes a request with resonable defaults", func() {
				Expect(serverMock.Params.Req.Header.Get("Accept")).To(Equal("application/json"))
				Expect(serverMock.Params.Req.Header.Get("Content-Type")).To(Equal("application/json"))
				Expect(serverMock.Params.Req.Header.Get("User-Agent")).To(Equal("kiln"))

				Expect(serverMock.Params.Req.Header.Get("Authorization")).To(BeEmpty())
			})
		})

		When("a UAA token is set", func() {
			BeforeEach(func() {
				pivnet.UAAAPIToken = "some-token"
			})

			It("makes a request with correct auth headers", func() {
				Expect(serverMock.Params.Req.Header.Get("Authorization")).To(Equal("Bearer some-token"))
			})
		})
	})

	When("fetching versions", func() {
		var (
			pivnet     fetcher.Pivnet
			serverMock *fakes.RoundTripper

			stemcellSlug string

			gotVersions []string
			gotErr      error
		)

		BeforeEach(func() {
			serverMock = &fakes.RoundTripper{}
			serverMock.Results.Res = &http.Response{}

			pivnet.Client = &http.Client{Transport: serverMock}
		})

		JustBeforeEach(func() {
			gotVersions, gotErr = pivnet.Versions(stemcellSlug)
		})

		When("fetching with an empty line as a string", func() {
			It("returns an error", func() {
				Expect(gotErr).To(Equal(fetcher.ErrProductSlugMustNotBeEmpty))
				Expect(gotVersions).To(BeNil())
			})
		})

		When("fetching versions for supported stemcells", func() {
			BeforeEach(func() {
				stemcellSlug = "some-stemcell"
			})
			When("the request fails", func() {
				BeforeEach(func() {
					serverMock.Results.Res = nil
					serverMock.Results.Err = errors.New("some-error")
				})
				It("returns an error", func() {
					Expect(gotErr).To(MatchError(ContainSubstring("some-error")))
				})
			})
			When("the json parsing fails", func() {
				BeforeEach(func() {
					serverMock.Results.Res.Body = fakes.NewReadCloser("{")
					serverMock.Results.Res.StatusCode = http.StatusOK
					serverMock.Results.Err = nil
				})
				It("returns an error", func() {
					Expect(gotErr).To(MatchError(ContainSubstring("unexpected end of JSON input")))
				})
			})
			When("the response body could not be read", func() {
				BeforeEach(func() {
					rc := &fakes.ReadCloser{}
					rc.ReadCall.Returns.Err = errors.New("some-error")
					serverMock.Results.Res.Body = rc
					serverMock.Results.Res.StatusCode = http.StatusOK
					serverMock.Results.Err = nil
				})
				It("returns an error", func() {
					Expect(gotErr).To(MatchError(ContainSubstring("some-error")))
				})
			})
			When("the request is not a success", func() {
				BeforeEach(func() {
					serverMock.Results.Res.Body = fakes.NewReadCloser(`foo`)
					serverMock.Results.Res.StatusCode = http.StatusTeapot
					serverMock.Results.Res.Status = http.StatusText(http.StatusTeapot)
					serverMock.Results.Err = nil
				})
				It("returns an error", func() {
					Expect(gotErr).To(MatchError(ContainSubstring("request was not successful, response had status")))
				})
			})
			When("the json parsing fails", func() {
				BeforeEach(func() {
					serverMock.Results.Res.Body = fakes.NewReadCloser(`{"releases": [{"version": "1.0"}, {"version": "2.1"}]}`)
					serverMock.Results.Res.StatusCode = http.StatusOK
					serverMock.Results.Err = nil
				})
				It("returns the versions", func() {
					Expect(gotErr).NotTo(HaveOccurred())
					Expect(gotVersions).To(Equal([]string{"1.0", "2.1"}))
				})
			})
		})
	})
})
