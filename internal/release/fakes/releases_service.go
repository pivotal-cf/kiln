// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"context"
	"sync"

	"github.com/google/go-github/v40/github"
	"github.com/pivotal-cf/kiln/pkg/component"
)

type ReleaseService struct {
	ListReleasesStub        func(context.Context, string, string, *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
	listReleasesMutex       sync.RWMutex
	listReleasesArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 *github.ListOptions
	}
	listReleasesReturns struct {
		result1 []*github.RepositoryRelease
		result2 *github.Response
		result3 error
	}
	listReleasesReturnsOnCall map[int]struct {
		result1 []*github.RepositoryRelease
		result2 *github.Response
		result3 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *ReleaseService) ListReleases(arg1 context.Context, arg2 string, arg3 string, arg4 *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	fake.listReleasesMutex.Lock()
	ret, specificReturn := fake.listReleasesReturnsOnCall[len(fake.listReleasesArgsForCall)]
	fake.listReleasesArgsForCall = append(fake.listReleasesArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 *github.ListOptions
	}{arg1, arg2, arg3, arg4})
	stub := fake.ListReleasesStub
	fakeReturns := fake.listReleasesReturns
	fake.recordInvocation("ListReleases", []interface{}{arg1, arg2, arg3, arg4})
	fake.listReleasesMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *ReleaseService) ListReleasesCallCount() int {
	fake.listReleasesMutex.RLock()
	defer fake.listReleasesMutex.RUnlock()
	return len(fake.listReleasesArgsForCall)
}

func (fake *ReleaseService) ListReleasesCalls(stub func(context.Context, string, string, *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)) {
	fake.listReleasesMutex.Lock()
	defer fake.listReleasesMutex.Unlock()
	fake.ListReleasesStub = stub
}

func (fake *ReleaseService) ListReleasesArgsForCall(i int) (context.Context, string, string, *github.ListOptions) {
	fake.listReleasesMutex.RLock()
	defer fake.listReleasesMutex.RUnlock()
	argsForCall := fake.listReleasesArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *ReleaseService) ListReleasesReturns(result1 []*github.RepositoryRelease, result2 *github.Response, result3 error) {
	fake.listReleasesMutex.Lock()
	defer fake.listReleasesMutex.Unlock()
	fake.ListReleasesStub = nil
	fake.listReleasesReturns = struct {
		result1 []*github.RepositoryRelease
		result2 *github.Response
		result3 error
	}{result1, result2, result3}
}

func (fake *ReleaseService) ListReleasesReturnsOnCall(i int, result1 []*github.RepositoryRelease, result2 *github.Response, result3 error) {
	fake.listReleasesMutex.Lock()
	defer fake.listReleasesMutex.Unlock()
	fake.ListReleasesStub = nil
	if fake.listReleasesReturnsOnCall == nil {
		fake.listReleasesReturnsOnCall = make(map[int]struct {
			result1 []*github.RepositoryRelease
			result2 *github.Response
			result3 error
		})
	}
	fake.listReleasesReturnsOnCall[i] = struct {
		result1 []*github.RepositoryRelease
		result2 *github.Response
		result3 error
	}{result1, result2, result3}
}

func (fake *ReleaseService) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.listReleasesMutex.RLock()
	defer fake.listReleasesMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *ReleaseService) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ component.RepositoryReleaseLister = new(ReleaseService)
