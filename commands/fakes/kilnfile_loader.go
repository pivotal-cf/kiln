// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	"sync"

	"github.com/pivotal-cf/kiln/commands"
	billy "gopkg.in/src-d/go-billy.v4"
)

type KilnfileLoader struct {
	LoadKilnfilesStub        func(billy.Filesystem, string, []string, []string) (cargo2.Kilnfile, cargo2.KilnfileLock, error)
	loadKilnfilesMutex       sync.RWMutex
	loadKilnfilesArgsForCall []struct {
		arg1 billy.Filesystem
		arg2 string
		arg3 []string
		arg4 []string
	}
	loadKilnfilesReturns struct {
		result1 cargo2.Kilnfile
		result2 cargo2.KilnfileLock
		result3 error
	}
	loadKilnfilesReturnsOnCall map[int]struct {
		result1 cargo2.Kilnfile
		result2 cargo2.KilnfileLock
		result3 error
	}
	SaveKilnfileLockStub        func(billy.Filesystem, string, cargo2.KilnfileLock) error
	saveKilnfileLockMutex       sync.RWMutex
	saveKilnfileLockArgsForCall []struct {
		arg1 billy.Filesystem
		arg2 string
		arg3 cargo2.KilnfileLock
	}
	saveKilnfileLockReturns struct {
		result1 error
	}
	saveKilnfileLockReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *KilnfileLoader) LoadKilnfiles(arg1 billy.Filesystem, arg2 string, arg3 []string, arg4 []string) (cargo2.Kilnfile, cargo2.KilnfileLock, error) {
	var arg3Copy []string
	if arg3 != nil {
		arg3Copy = make([]string, len(arg3))
		copy(arg3Copy, arg3)
	}
	var arg4Copy []string
	if arg4 != nil {
		arg4Copy = make([]string, len(arg4))
		copy(arg4Copy, arg4)
	}
	fake.loadKilnfilesMutex.Lock()
	ret, specificReturn := fake.loadKilnfilesReturnsOnCall[len(fake.loadKilnfilesArgsForCall)]
	fake.loadKilnfilesArgsForCall = append(fake.loadKilnfilesArgsForCall, struct {
		arg1 billy.Filesystem
		arg2 string
		arg3 []string
		arg4 []string
	}{arg1, arg2, arg3Copy, arg4Copy})
	fake.recordInvocation("LoadKilnfiles", []interface{}{arg1, arg2, arg3Copy, arg4Copy})
	fake.loadKilnfilesMutex.Unlock()
	if fake.LoadKilnfilesStub != nil {
		return fake.LoadKilnfilesStub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	fakeReturns := fake.loadKilnfilesReturns
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *KilnfileLoader) LoadKilnfilesCallCount() int {
	fake.loadKilnfilesMutex.RLock()
	defer fake.loadKilnfilesMutex.RUnlock()
	return len(fake.loadKilnfilesArgsForCall)
}

func (fake *KilnfileLoader) LoadKilnfilesCalls(stub func(billy.Filesystem, string, []string, []string) (cargo2.Kilnfile, cargo2.KilnfileLock, error)) {
	fake.loadKilnfilesMutex.Lock()
	defer fake.loadKilnfilesMutex.Unlock()
	fake.LoadKilnfilesStub = stub
}

func (fake *KilnfileLoader) LoadKilnfilesArgsForCall(i int) (billy.Filesystem, string, []string, []string) {
	fake.loadKilnfilesMutex.RLock()
	defer fake.loadKilnfilesMutex.RUnlock()
	argsForCall := fake.loadKilnfilesArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *KilnfileLoader) LoadKilnfilesReturns(result1 cargo2.Kilnfile, result2 cargo2.KilnfileLock, result3 error) {
	fake.loadKilnfilesMutex.Lock()
	defer fake.loadKilnfilesMutex.Unlock()
	fake.LoadKilnfilesStub = nil
	fake.loadKilnfilesReturns = struct {
		result1 cargo2.Kilnfile
		result2 cargo2.KilnfileLock
		result3 error
	}{result1, result2, result3}
}

func (fake *KilnfileLoader) LoadKilnfilesReturnsOnCall(i int, result1 cargo2.Kilnfile, result2 cargo2.KilnfileLock, result3 error) {
	fake.loadKilnfilesMutex.Lock()
	defer fake.loadKilnfilesMutex.Unlock()
	fake.LoadKilnfilesStub = nil
	if fake.loadKilnfilesReturnsOnCall == nil {
		fake.loadKilnfilesReturnsOnCall = make(map[int]struct {
			result1 cargo2.Kilnfile
			result2 cargo2.KilnfileLock
			result3 error
		})
	}
	fake.loadKilnfilesReturnsOnCall[i] = struct {
		result1 cargo2.Kilnfile
		result2 cargo2.KilnfileLock
		result3 error
	}{result1, result2, result3}
}

func (fake *KilnfileLoader) SaveKilnfileLock(arg1 billy.Filesystem, arg2 string, arg3 cargo2.KilnfileLock) error {
	fake.saveKilnfileLockMutex.Lock()
	ret, specificReturn := fake.saveKilnfileLockReturnsOnCall[len(fake.saveKilnfileLockArgsForCall)]
	fake.saveKilnfileLockArgsForCall = append(fake.saveKilnfileLockArgsForCall, struct {
		arg1 billy.Filesystem
		arg2 string
		arg3 cargo2.KilnfileLock
	}{arg1, arg2, arg3})
	fake.recordInvocation("SaveKilnfileLock", []interface{}{arg1, arg2, arg3})
	fake.saveKilnfileLockMutex.Unlock()
	if fake.SaveKilnfileLockStub != nil {
		return fake.SaveKilnfileLockStub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	fakeReturns := fake.saveKilnfileLockReturns
	return fakeReturns.result1
}

func (fake *KilnfileLoader) SaveKilnfileLockCallCount() int {
	fake.saveKilnfileLockMutex.RLock()
	defer fake.saveKilnfileLockMutex.RUnlock()
	return len(fake.saveKilnfileLockArgsForCall)
}

func (fake *KilnfileLoader) SaveKilnfileLockCalls(stub func(billy.Filesystem, string, cargo2.KilnfileLock) error) {
	fake.saveKilnfileLockMutex.Lock()
	defer fake.saveKilnfileLockMutex.Unlock()
	fake.SaveKilnfileLockStub = stub
}

func (fake *KilnfileLoader) SaveKilnfileLockArgsForCall(i int) (billy.Filesystem, string, cargo2.KilnfileLock) {
	fake.saveKilnfileLockMutex.RLock()
	defer fake.saveKilnfileLockMutex.RUnlock()
	argsForCall := fake.saveKilnfileLockArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *KilnfileLoader) SaveKilnfileLockReturns(result1 error) {
	fake.saveKilnfileLockMutex.Lock()
	defer fake.saveKilnfileLockMutex.Unlock()
	fake.SaveKilnfileLockStub = nil
	fake.saveKilnfileLockReturns = struct {
		result1 error
	}{result1}
}

func (fake *KilnfileLoader) SaveKilnfileLockReturnsOnCall(i int, result1 error) {
	fake.saveKilnfileLockMutex.Lock()
	defer fake.saveKilnfileLockMutex.Unlock()
	fake.SaveKilnfileLockStub = nil
	if fake.saveKilnfileLockReturnsOnCall == nil {
		fake.saveKilnfileLockReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.saveKilnfileLockReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *KilnfileLoader) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.loadKilnfilesMutex.RLock()
	defer fake.loadKilnfilesMutex.RUnlock()
	fake.saveKilnfileLockMutex.RLock()
	defer fake.saveKilnfileLockMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *KilnfileLoader) recordInvocation(key string, args []interface{}) {
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

var _ commands.KilnfileLoader = new(KilnfileLoader)
