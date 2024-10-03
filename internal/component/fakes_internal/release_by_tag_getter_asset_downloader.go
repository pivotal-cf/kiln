// Code generated by counterfeiter. DO NOT EDIT.
package fakes_internal

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/google/go-github/v65/github"
)

type ReleaseByTagGetterAssetDownloader struct {
	DownloadReleaseAssetStub        func(context.Context, string, string, int64, *http.Client) (io.ReadCloser, string, error)
	downloadReleaseAssetMutex       sync.RWMutex
	downloadReleaseAssetArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 int64
		arg5 *http.Client
	}
	downloadReleaseAssetReturns struct {
		result1 io.ReadCloser
		result2 string
		result3 error
	}
	downloadReleaseAssetReturnsOnCall map[int]struct {
		result1 io.ReadCloser
		result2 string
		result3 error
	}
	GetReleaseByTagStub        func(context.Context, string, string, string) (*github.RepositoryRelease, *github.Response, error)
	getReleaseByTagMutex       sync.RWMutex
	getReleaseByTagArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 string
	}
	getReleaseByTagReturns struct {
		result1 *github.RepositoryRelease
		result2 *github.Response
		result3 error
	}
	getReleaseByTagReturnsOnCall map[int]struct {
		result1 *github.RepositoryRelease
		result2 *github.Response
		result3 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *ReleaseByTagGetterAssetDownloader) DownloadReleaseAsset(arg1 context.Context, arg2 string, arg3 string, arg4 int64, arg5 *http.Client) (io.ReadCloser, string, error) {
	fake.downloadReleaseAssetMutex.Lock()
	ret, specificReturn := fake.downloadReleaseAssetReturnsOnCall[len(fake.downloadReleaseAssetArgsForCall)]
	fake.downloadReleaseAssetArgsForCall = append(fake.downloadReleaseAssetArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 int64
		arg5 *http.Client
	}{arg1, arg2, arg3, arg4, arg5})
	stub := fake.DownloadReleaseAssetStub
	fakeReturns := fake.downloadReleaseAssetReturns
	fake.recordInvocation("DownloadReleaseAsset", []any{arg1, arg2, arg3, arg4, arg5})
	fake.downloadReleaseAssetMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4, arg5)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *ReleaseByTagGetterAssetDownloader) DownloadReleaseAssetCallCount() int {
	fake.downloadReleaseAssetMutex.RLock()
	defer fake.downloadReleaseAssetMutex.RUnlock()
	return len(fake.downloadReleaseAssetArgsForCall)
}

func (fake *ReleaseByTagGetterAssetDownloader) DownloadReleaseAssetCalls(stub func(context.Context, string, string, int64, *http.Client) (io.ReadCloser, string, error)) {
	fake.downloadReleaseAssetMutex.Lock()
	defer fake.downloadReleaseAssetMutex.Unlock()
	fake.DownloadReleaseAssetStub = stub
}

func (fake *ReleaseByTagGetterAssetDownloader) DownloadReleaseAssetArgsForCall(i int) (context.Context, string, string, int64, *http.Client) {
	fake.downloadReleaseAssetMutex.RLock()
	defer fake.downloadReleaseAssetMutex.RUnlock()
	argsForCall := fake.downloadReleaseAssetArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5
}

func (fake *ReleaseByTagGetterAssetDownloader) DownloadReleaseAssetReturns(result1 io.ReadCloser, result2 string, result3 error) {
	fake.downloadReleaseAssetMutex.Lock()
	defer fake.downloadReleaseAssetMutex.Unlock()
	fake.DownloadReleaseAssetStub = nil
	fake.downloadReleaseAssetReturns = struct {
		result1 io.ReadCloser
		result2 string
		result3 error
	}{result1, result2, result3}
}

func (fake *ReleaseByTagGetterAssetDownloader) DownloadReleaseAssetReturnsOnCall(i int, result1 io.ReadCloser, result2 string, result3 error) {
	fake.downloadReleaseAssetMutex.Lock()
	defer fake.downloadReleaseAssetMutex.Unlock()
	fake.DownloadReleaseAssetStub = nil
	if fake.downloadReleaseAssetReturnsOnCall == nil {
		fake.downloadReleaseAssetReturnsOnCall = make(map[int]struct {
			result1 io.ReadCloser
			result2 string
			result3 error
		})
	}
	fake.downloadReleaseAssetReturnsOnCall[i] = struct {
		result1 io.ReadCloser
		result2 string
		result3 error
	}{result1, result2, result3}
}

func (fake *ReleaseByTagGetterAssetDownloader) GetReleaseByTag(arg1 context.Context, arg2 string, arg3 string, arg4 string) (*github.RepositoryRelease, *github.Response, error) {
	fake.getReleaseByTagMutex.Lock()
	ret, specificReturn := fake.getReleaseByTagReturnsOnCall[len(fake.getReleaseByTagArgsForCall)]
	fake.getReleaseByTagArgsForCall = append(fake.getReleaseByTagArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 string
	}{arg1, arg2, arg3, arg4})
	stub := fake.GetReleaseByTagStub
	fakeReturns := fake.getReleaseByTagReturns
	fake.recordInvocation("GetReleaseByTag", []any{arg1, arg2, arg3, arg4})
	fake.getReleaseByTagMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *ReleaseByTagGetterAssetDownloader) GetReleaseByTagCallCount() int {
	fake.getReleaseByTagMutex.RLock()
	defer fake.getReleaseByTagMutex.RUnlock()
	return len(fake.getReleaseByTagArgsForCall)
}

func (fake *ReleaseByTagGetterAssetDownloader) GetReleaseByTagCalls(stub func(context.Context, string, string, string) (*github.RepositoryRelease, *github.Response, error)) {
	fake.getReleaseByTagMutex.Lock()
	defer fake.getReleaseByTagMutex.Unlock()
	fake.GetReleaseByTagStub = stub
}

func (fake *ReleaseByTagGetterAssetDownloader) GetReleaseByTagArgsForCall(i int) (context.Context, string, string, string) {
	fake.getReleaseByTagMutex.RLock()
	defer fake.getReleaseByTagMutex.RUnlock()
	argsForCall := fake.getReleaseByTagArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *ReleaseByTagGetterAssetDownloader) GetReleaseByTagReturns(result1 *github.RepositoryRelease, result2 *github.Response, result3 error) {
	fake.getReleaseByTagMutex.Lock()
	defer fake.getReleaseByTagMutex.Unlock()
	fake.GetReleaseByTagStub = nil
	fake.getReleaseByTagReturns = struct {
		result1 *github.RepositoryRelease
		result2 *github.Response
		result3 error
	}{result1, result2, result3}
}

func (fake *ReleaseByTagGetterAssetDownloader) GetReleaseByTagReturnsOnCall(i int, result1 *github.RepositoryRelease, result2 *github.Response, result3 error) {
	fake.getReleaseByTagMutex.Lock()
	defer fake.getReleaseByTagMutex.Unlock()
	fake.GetReleaseByTagStub = nil
	if fake.getReleaseByTagReturnsOnCall == nil {
		fake.getReleaseByTagReturnsOnCall = make(map[int]struct {
			result1 *github.RepositoryRelease
			result2 *github.Response
			result3 error
		})
	}
	fake.getReleaseByTagReturnsOnCall[i] = struct {
		result1 *github.RepositoryRelease
		result2 *github.Response
		result3 error
	}{result1, result2, result3}
}

func (fake *ReleaseByTagGetterAssetDownloader) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.downloadReleaseAssetMutex.RLock()
	defer fake.downloadReleaseAssetMutex.RUnlock()
	fake.getReleaseByTagMutex.RLock()
	defer fake.getReleaseByTagMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *ReleaseByTagGetterAssetDownloader) recordInvocation(key string, args []any) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]any{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]any{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
