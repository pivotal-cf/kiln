package fakes

import "net/http"

type RoundTripper struct {
	Params struct {
		Req *http.Request
	}
	Results struct {
		Res *http.Response
		Err error
	}
}

func (mock *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	mock.Params.Req = req
	return mock.Results.Res, mock.Results.Err
}
