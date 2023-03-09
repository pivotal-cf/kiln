// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type ReleaseStorage struct {
	ConfigurationStub        func() cargo.ReleaseSourceConfig
	configurationMutex       sync.RWMutex
	configurationArgsForCall []struct {
	}
	configurationReturns struct {
		result1 cargo.ReleaseSourceConfig
	}
	configurationReturnsOnCall map[int]struct {
		result1 cargo.ReleaseSourceConfig
	}
	DownloadReleaseStub        func(context.Context, string, cargo.ComponentLock) (component.Local, error)
	downloadReleaseMutex       sync.RWMutex
	downloadReleaseArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 cargo.ComponentLock
	}
	downloadReleaseReturns struct {
		result1 component.Local
		result2 error
	}
	downloadReleaseReturnsOnCall map[int]struct {
		result1 component.Local
		result2 error
	}
	FindReleaseVersionStub        func(context.Context, cargo.ComponentSpec, bool) (cargo.ComponentLock, error)
	findReleaseVersionMutex       sync.RWMutex
	findReleaseVersionArgsForCall []struct {
		arg1 context.Context
		arg2 cargo.ComponentSpec
		arg3 bool
	}
	findReleaseVersionReturns struct {
		result1 cargo.ComponentLock
		result2 error
	}
	findReleaseVersionReturnsOnCall map[int]struct {
		result1 cargo.ComponentLock
		result2 error
	}
	GetMatchedReleaseStub        func(context.Context, cargo.ComponentSpec) (cargo.ComponentLock, error)
	getMatchedReleaseMutex       sync.RWMutex
	getMatchedReleaseArgsForCall []struct {
		arg1 context.Context
		arg2 cargo.ComponentSpec
	}
	getMatchedReleaseReturns struct {
		result1 cargo.ComponentLock
		result2 error
	}
	getMatchedReleaseReturnsOnCall map[int]struct {
		result1 cargo.ComponentLock
		result2 error
	}
	UploadReleaseStub        func(cargo.ComponentSpec, io.Reader) (cargo.ComponentLock, error)
	uploadReleaseMutex       sync.RWMutex
	uploadReleaseArgsForCall []struct {
		arg1 cargo.ComponentSpec
		arg2 io.Reader
	}
	uploadReleaseReturns struct {
		result1 cargo.ComponentLock
		result2 error
	}
	uploadReleaseReturnsOnCall map[int]struct {
		result1 cargo.ComponentLock
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *ReleaseStorage) Configuration() cargo.ReleaseSourceConfig {
	fake.configurationMutex.Lock()
	ret, specificReturn := fake.configurationReturnsOnCall[len(fake.configurationArgsForCall)]
	fake.configurationArgsForCall = append(fake.configurationArgsForCall, struct {
	}{})
	stub := fake.ConfigurationStub
	fakeReturns := fake.configurationReturns
	fake.recordInvocation("Configuration", []interface{}{})
	fake.configurationMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *ReleaseStorage) ConfigurationCallCount() int {
	fake.configurationMutex.RLock()
	defer fake.configurationMutex.RUnlock()
	return len(fake.configurationArgsForCall)
}

func (fake *ReleaseStorage) ConfigurationCalls(stub func() cargo.ReleaseSourceConfig) {
	fake.configurationMutex.Lock()
	defer fake.configurationMutex.Unlock()
	fake.ConfigurationStub = stub
}

func (fake *ReleaseStorage) ConfigurationReturns(result1 cargo.ReleaseSourceConfig) {
	fake.configurationMutex.Lock()
	defer fake.configurationMutex.Unlock()
	fake.ConfigurationStub = nil
	fake.configurationReturns = struct {
		result1 cargo.ReleaseSourceConfig
	}{result1}
}

func (fake *ReleaseStorage) ConfigurationReturnsOnCall(i int, result1 cargo.ReleaseSourceConfig) {
	fake.configurationMutex.Lock()
	defer fake.configurationMutex.Unlock()
	fake.ConfigurationStub = nil
	if fake.configurationReturnsOnCall == nil {
		fake.configurationReturnsOnCall = make(map[int]struct {
			result1 cargo.ReleaseSourceConfig
		})
	}
	fake.configurationReturnsOnCall[i] = struct {
		result1 cargo.ReleaseSourceConfig
	}{result1}
}

func (fake *ReleaseStorage) DownloadRelease(arg1 context.Context, arg2 string, arg3 cargo.ComponentLock) (component.Local, error) {
	fake.downloadReleaseMutex.Lock()
	ret, specificReturn := fake.downloadReleaseReturnsOnCall[len(fake.downloadReleaseArgsForCall)]
	fake.downloadReleaseArgsForCall = append(fake.downloadReleaseArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 cargo.ComponentLock
	}{arg1, arg2, arg3})
	stub := fake.DownloadReleaseStub
	fakeReturns := fake.downloadReleaseReturns
	fake.recordInvocation("DownloadRelease", []interface{}{arg1, arg2, arg3})
	fake.downloadReleaseMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *ReleaseStorage) DownloadReleaseCallCount() int {
	fake.downloadReleaseMutex.RLock()
	defer fake.downloadReleaseMutex.RUnlock()
	return len(fake.downloadReleaseArgsForCall)
}

func (fake *ReleaseStorage) DownloadReleaseCalls(stub func(context.Context, string, cargo.ComponentLock) (component.Local, error)) {
	fake.downloadReleaseMutex.Lock()
	defer fake.downloadReleaseMutex.Unlock()
	fake.DownloadReleaseStub = stub
}

func (fake *ReleaseStorage) DownloadReleaseArgsForCall(i int) (context.Context, string, cargo.ComponentLock) {
	fake.downloadReleaseMutex.RLock()
	defer fake.downloadReleaseMutex.RUnlock()
	argsForCall := fake.downloadReleaseArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *ReleaseStorage) DownloadReleaseReturns(result1 component.Local, result2 error) {
	fake.downloadReleaseMutex.Lock()
	defer fake.downloadReleaseMutex.Unlock()
	fake.DownloadReleaseStub = nil
	fake.downloadReleaseReturns = struct {
		result1 component.Local
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) DownloadReleaseReturnsOnCall(i int, result1 component.Local, result2 error) {
	fake.downloadReleaseMutex.Lock()
	defer fake.downloadReleaseMutex.Unlock()
	fake.DownloadReleaseStub = nil
	if fake.downloadReleaseReturnsOnCall == nil {
		fake.downloadReleaseReturnsOnCall = make(map[int]struct {
			result1 component.Local
			result2 error
		})
	}
	fake.downloadReleaseReturnsOnCall[i] = struct {
		result1 component.Local
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) FindReleaseVersion(arg1 context.Context, arg2 cargo.ComponentSpec, arg3 bool) (cargo.ComponentLock, error) {
	fake.findReleaseVersionMutex.Lock()
	ret, specificReturn := fake.findReleaseVersionReturnsOnCall[len(fake.findReleaseVersionArgsForCall)]
	fake.findReleaseVersionArgsForCall = append(fake.findReleaseVersionArgsForCall, struct {
		arg1 context.Context
		arg2 cargo.ComponentSpec
		arg3 bool
	}{arg1, arg2, arg3})
	stub := fake.FindReleaseVersionStub
	fakeReturns := fake.findReleaseVersionReturns
	fake.recordInvocation("FindReleaseVersion", []interface{}{arg1, arg2, arg3})
	fake.findReleaseVersionMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *ReleaseStorage) FindReleaseVersionCallCount() int {
	fake.findReleaseVersionMutex.RLock()
	defer fake.findReleaseVersionMutex.RUnlock()
	return len(fake.findReleaseVersionArgsForCall)
}

func (fake *ReleaseStorage) FindReleaseVersionCalls(stub func(context.Context, cargo.ComponentSpec, bool) (cargo.ComponentLock, error)) {
	fake.findReleaseVersionMutex.Lock()
	defer fake.findReleaseVersionMutex.Unlock()
	fake.FindReleaseVersionStub = stub
}

func (fake *ReleaseStorage) FindReleaseVersionArgsForCall(i int) (context.Context, cargo.ComponentSpec, bool) {
	fake.findReleaseVersionMutex.RLock()
	defer fake.findReleaseVersionMutex.RUnlock()
	argsForCall := fake.findReleaseVersionArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *ReleaseStorage) FindReleaseVersionReturns(result1 cargo.ComponentLock, result2 error) {
	fake.findReleaseVersionMutex.Lock()
	defer fake.findReleaseVersionMutex.Unlock()
	fake.FindReleaseVersionStub = nil
	fake.findReleaseVersionReturns = struct {
		result1 cargo.ComponentLock
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) FindReleaseVersionReturnsOnCall(i int, result1 cargo.ComponentLock, result2 error) {
	fake.findReleaseVersionMutex.Lock()
	defer fake.findReleaseVersionMutex.Unlock()
	fake.FindReleaseVersionStub = nil
	if fake.findReleaseVersionReturnsOnCall == nil {
		fake.findReleaseVersionReturnsOnCall = make(map[int]struct {
			result1 cargo.ComponentLock
			result2 error
		})
	}
	fake.findReleaseVersionReturnsOnCall[i] = struct {
		result1 cargo.ComponentLock
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) GetMatchedRelease(arg1 context.Context, arg2 cargo.ComponentSpec) (cargo.ComponentLock, error) {
	fake.getMatchedReleaseMutex.Lock()
	ret, specificReturn := fake.getMatchedReleaseReturnsOnCall[len(fake.getMatchedReleaseArgsForCall)]
	fake.getMatchedReleaseArgsForCall = append(fake.getMatchedReleaseArgsForCall, struct {
		arg1 context.Context
		arg2 cargo.ComponentSpec
	}{arg1, arg2})
	stub := fake.GetMatchedReleaseStub
	fakeReturns := fake.getMatchedReleaseReturns
	fake.recordInvocation("GetMatchedRelease", []interface{}{arg1, arg2})
	fake.getMatchedReleaseMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *ReleaseStorage) GetMatchedReleaseCallCount() int {
	fake.getMatchedReleaseMutex.RLock()
	defer fake.getMatchedReleaseMutex.RUnlock()
	return len(fake.getMatchedReleaseArgsForCall)
}

func (fake *ReleaseStorage) GetMatchedReleaseCalls(stub func(context.Context, cargo.ComponentSpec) (cargo.ComponentLock, error)) {
	fake.getMatchedReleaseMutex.Lock()
	defer fake.getMatchedReleaseMutex.Unlock()
	fake.GetMatchedReleaseStub = stub
}

func (fake *ReleaseStorage) GetMatchedReleaseArgsForCall(i int) (context.Context, cargo.ComponentSpec) {
	fake.getMatchedReleaseMutex.RLock()
	defer fake.getMatchedReleaseMutex.RUnlock()
	argsForCall := fake.getMatchedReleaseArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *ReleaseStorage) GetMatchedReleaseReturns(result1 cargo.ComponentLock, result2 error) {
	fake.getMatchedReleaseMutex.Lock()
	defer fake.getMatchedReleaseMutex.Unlock()
	fake.GetMatchedReleaseStub = nil
	fake.getMatchedReleaseReturns = struct {
		result1 cargo.ComponentLock
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) GetMatchedReleaseReturnsOnCall(i int, result1 cargo.ComponentLock, result2 error) {
	fake.getMatchedReleaseMutex.Lock()
	defer fake.getMatchedReleaseMutex.Unlock()
	fake.GetMatchedReleaseStub = nil
	if fake.getMatchedReleaseReturnsOnCall == nil {
		fake.getMatchedReleaseReturnsOnCall = make(map[int]struct {
			result1 cargo.ComponentLock
			result2 error
		})
	}
	fake.getMatchedReleaseReturnsOnCall[i] = struct {
		result1 cargo.ComponentLock
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) UploadRelease(arg1 cargo.ComponentSpec, arg2 io.Reader) (cargo.ComponentLock, error) {
	fake.uploadReleaseMutex.Lock()
	ret, specificReturn := fake.uploadReleaseReturnsOnCall[len(fake.uploadReleaseArgsForCall)]
	fake.uploadReleaseArgsForCall = append(fake.uploadReleaseArgsForCall, struct {
		arg1 cargo.ComponentSpec
		arg2 io.Reader
	}{arg1, arg2})
	stub := fake.UploadReleaseStub
	fakeReturns := fake.uploadReleaseReturns
	fake.recordInvocation("UploadRelease", []interface{}{arg1, arg2})
	fake.uploadReleaseMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *ReleaseStorage) UploadReleaseCallCount() int {
	fake.uploadReleaseMutex.RLock()
	defer fake.uploadReleaseMutex.RUnlock()
	return len(fake.uploadReleaseArgsForCall)
}

func (fake *ReleaseStorage) UploadReleaseCalls(stub func(cargo.ComponentSpec, io.Reader) (cargo.ComponentLock, error)) {
	fake.uploadReleaseMutex.Lock()
	defer fake.uploadReleaseMutex.Unlock()
	fake.UploadReleaseStub = stub
}

func (fake *ReleaseStorage) UploadReleaseArgsForCall(i int) (cargo.ComponentSpec, io.Reader) {
	fake.uploadReleaseMutex.RLock()
	defer fake.uploadReleaseMutex.RUnlock()
	argsForCall := fake.uploadReleaseArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *ReleaseStorage) UploadReleaseReturns(result1 cargo.ComponentLock, result2 error) {
	fake.uploadReleaseMutex.Lock()
	defer fake.uploadReleaseMutex.Unlock()
	fake.UploadReleaseStub = nil
	fake.uploadReleaseReturns = struct {
		result1 cargo.ComponentLock
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) UploadReleaseReturnsOnCall(i int, result1 cargo.ComponentLock, result2 error) {
	fake.uploadReleaseMutex.Lock()
	defer fake.uploadReleaseMutex.Unlock()
	fake.UploadReleaseStub = nil
	if fake.uploadReleaseReturnsOnCall == nil {
		fake.uploadReleaseReturnsOnCall = make(map[int]struct {
			result1 cargo.ComponentLock
			result2 error
		})
	}
	fake.uploadReleaseReturnsOnCall[i] = struct {
		result1 cargo.ComponentLock
		result2 error
	}{result1, result2}
}

func (fake *ReleaseStorage) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.configurationMutex.RLock()
	defer fake.configurationMutex.RUnlock()
	fake.downloadReleaseMutex.RLock()
	defer fake.downloadReleaseMutex.RUnlock()
	fake.findReleaseVersionMutex.RLock()
	defer fake.findReleaseVersionMutex.RUnlock()
	fake.getMatchedReleaseMutex.RLock()
	defer fake.getMatchedReleaseMutex.RUnlock()
	fake.uploadReleaseMutex.RLock()
	defer fake.uploadReleaseMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *ReleaseStorage) recordInvocation(key string, args []interface{}) {
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

var _ commands.ReleaseStorage = new(ReleaseStorage)
