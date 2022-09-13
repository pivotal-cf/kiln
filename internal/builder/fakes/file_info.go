// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"io/fs"
	"os"
	"sync"
	"time"
)

type FileInfo struct {
	IsDirStub        func() bool
	isDirMutex       sync.RWMutex
	isDirArgsForCall []struct {
	}
	isDirReturns struct {
		result1 bool
	}
	isDirReturnsOnCall map[int]struct {
		result1 bool
	}
	ModTimeStub        func() time.Time
	modTimeMutex       sync.RWMutex
	modTimeArgsForCall []struct {
	}
	modTimeReturns struct {
		result1 time.Time
	}
	modTimeReturnsOnCall map[int]struct {
		result1 time.Time
	}
	ModeStub        func() fs.FileMode
	modeMutex       sync.RWMutex
	modeArgsForCall []struct {
	}
	modeReturns struct {
		result1 fs.FileMode
	}
	modeReturnsOnCall map[int]struct {
		result1 fs.FileMode
	}
	NameStub        func() string
	nameMutex       sync.RWMutex
	nameArgsForCall []struct {
	}
	nameReturns struct {
		result1 string
	}
	nameReturnsOnCall map[int]struct {
		result1 string
	}
	SizeStub        func() int64
	sizeMutex       sync.RWMutex
	sizeArgsForCall []struct {
	}
	sizeReturns struct {
		result1 int64
	}
	sizeReturnsOnCall map[int]struct {
		result1 int64
	}
	SysStub        func() any
	sysMutex       sync.RWMutex
	sysArgsForCall []struct {
	}
	sysReturns struct {
		result1 any
	}
	sysReturnsOnCall map[int]struct {
		result1 any
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FileInfo) IsDir() bool {
	fake.isDirMutex.Lock()
	ret, specificReturn := fake.isDirReturnsOnCall[len(fake.isDirArgsForCall)]
	fake.isDirArgsForCall = append(fake.isDirArgsForCall, struct {
	}{})
	stub := fake.IsDirStub
	fakeReturns := fake.isDirReturns
	fake.recordInvocation("IsDir", []interface{}{})
	fake.isDirMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FileInfo) IsDirCallCount() int {
	fake.isDirMutex.RLock()
	defer fake.isDirMutex.RUnlock()
	return len(fake.isDirArgsForCall)
}

func (fake *FileInfo) IsDirCalls(stub func() bool) {
	fake.isDirMutex.Lock()
	defer fake.isDirMutex.Unlock()
	fake.IsDirStub = stub
}

func (fake *FileInfo) IsDirReturns(result1 bool) {
	fake.isDirMutex.Lock()
	defer fake.isDirMutex.Unlock()
	fake.IsDirStub = nil
	fake.isDirReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FileInfo) IsDirReturnsOnCall(i int, result1 bool) {
	fake.isDirMutex.Lock()
	defer fake.isDirMutex.Unlock()
	fake.IsDirStub = nil
	if fake.isDirReturnsOnCall == nil {
		fake.isDirReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.isDirReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FileInfo) ModTime() time.Time {
	fake.modTimeMutex.Lock()
	ret, specificReturn := fake.modTimeReturnsOnCall[len(fake.modTimeArgsForCall)]
	fake.modTimeArgsForCall = append(fake.modTimeArgsForCall, struct {
	}{})
	stub := fake.ModTimeStub
	fakeReturns := fake.modTimeReturns
	fake.recordInvocation("ModTime", []interface{}{})
	fake.modTimeMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FileInfo) ModTimeCallCount() int {
	fake.modTimeMutex.RLock()
	defer fake.modTimeMutex.RUnlock()
	return len(fake.modTimeArgsForCall)
}

func (fake *FileInfo) ModTimeCalls(stub func() time.Time) {
	fake.modTimeMutex.Lock()
	defer fake.modTimeMutex.Unlock()
	fake.ModTimeStub = stub
}

func (fake *FileInfo) ModTimeReturns(result1 time.Time) {
	fake.modTimeMutex.Lock()
	defer fake.modTimeMutex.Unlock()
	fake.ModTimeStub = nil
	fake.modTimeReturns = struct {
		result1 time.Time
	}{result1}
}

func (fake *FileInfo) ModTimeReturnsOnCall(i int, result1 time.Time) {
	fake.modTimeMutex.Lock()
	defer fake.modTimeMutex.Unlock()
	fake.ModTimeStub = nil
	if fake.modTimeReturnsOnCall == nil {
		fake.modTimeReturnsOnCall = make(map[int]struct {
			result1 time.Time
		})
	}
	fake.modTimeReturnsOnCall[i] = struct {
		result1 time.Time
	}{result1}
}

func (fake *FileInfo) Mode() fs.FileMode {
	fake.modeMutex.Lock()
	ret, specificReturn := fake.modeReturnsOnCall[len(fake.modeArgsForCall)]
	fake.modeArgsForCall = append(fake.modeArgsForCall, struct {
	}{})
	stub := fake.ModeStub
	fakeReturns := fake.modeReturns
	fake.recordInvocation("Mode", []interface{}{})
	fake.modeMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FileInfo) ModeCallCount() int {
	fake.modeMutex.RLock()
	defer fake.modeMutex.RUnlock()
	return len(fake.modeArgsForCall)
}

func (fake *FileInfo) ModeCalls(stub func() fs.FileMode) {
	fake.modeMutex.Lock()
	defer fake.modeMutex.Unlock()
	fake.ModeStub = stub
}

func (fake *FileInfo) ModeReturns(result1 fs.FileMode) {
	fake.modeMutex.Lock()
	defer fake.modeMutex.Unlock()
	fake.ModeStub = nil
	fake.modeReturns = struct {
		result1 fs.FileMode
	}{result1}
}

func (fake *FileInfo) ModeReturnsOnCall(i int, result1 fs.FileMode) {
	fake.modeMutex.Lock()
	defer fake.modeMutex.Unlock()
	fake.ModeStub = nil
	if fake.modeReturnsOnCall == nil {
		fake.modeReturnsOnCall = make(map[int]struct {
			result1 fs.FileMode
		})
	}
	fake.modeReturnsOnCall[i] = struct {
		result1 fs.FileMode
	}{result1}
}

func (fake *FileInfo) Name() string {
	fake.nameMutex.Lock()
	ret, specificReturn := fake.nameReturnsOnCall[len(fake.nameArgsForCall)]
	fake.nameArgsForCall = append(fake.nameArgsForCall, struct {
	}{})
	stub := fake.NameStub
	fakeReturns := fake.nameReturns
	fake.recordInvocation("Name", []interface{}{})
	fake.nameMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FileInfo) NameCallCount() int {
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	return len(fake.nameArgsForCall)
}

func (fake *FileInfo) NameCalls(stub func() string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = stub
}

func (fake *FileInfo) NameReturns(result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	fake.nameReturns = struct {
		result1 string
	}{result1}
}

func (fake *FileInfo) NameReturnsOnCall(i int, result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	if fake.nameReturnsOnCall == nil {
		fake.nameReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.nameReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FileInfo) Size() int64 {
	fake.sizeMutex.Lock()
	ret, specificReturn := fake.sizeReturnsOnCall[len(fake.sizeArgsForCall)]
	fake.sizeArgsForCall = append(fake.sizeArgsForCall, struct {
	}{})
	stub := fake.SizeStub
	fakeReturns := fake.sizeReturns
	fake.recordInvocation("Size", []interface{}{})
	fake.sizeMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FileInfo) SizeCallCount() int {
	fake.sizeMutex.RLock()
	defer fake.sizeMutex.RUnlock()
	return len(fake.sizeArgsForCall)
}

func (fake *FileInfo) SizeCalls(stub func() int64) {
	fake.sizeMutex.Lock()
	defer fake.sizeMutex.Unlock()
	fake.SizeStub = stub
}

func (fake *FileInfo) SizeReturns(result1 int64) {
	fake.sizeMutex.Lock()
	defer fake.sizeMutex.Unlock()
	fake.SizeStub = nil
	fake.sizeReturns = struct {
		result1 int64
	}{result1}
}

func (fake *FileInfo) SizeReturnsOnCall(i int, result1 int64) {
	fake.sizeMutex.Lock()
	defer fake.sizeMutex.Unlock()
	fake.SizeStub = nil
	if fake.sizeReturnsOnCall == nil {
		fake.sizeReturnsOnCall = make(map[int]struct {
			result1 int64
		})
	}
	fake.sizeReturnsOnCall[i] = struct {
		result1 int64
	}{result1}
}

func (fake *FileInfo) Sys() any {
	fake.sysMutex.Lock()
	ret, specificReturn := fake.sysReturnsOnCall[len(fake.sysArgsForCall)]
	fake.sysArgsForCall = append(fake.sysArgsForCall, struct {
	}{})
	stub := fake.SysStub
	fakeReturns := fake.sysReturns
	fake.recordInvocation("Sys", []interface{}{})
	fake.sysMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FileInfo) SysCallCount() int {
	fake.sysMutex.RLock()
	defer fake.sysMutex.RUnlock()
	return len(fake.sysArgsForCall)
}

func (fake *FileInfo) SysCalls(stub func() any) {
	fake.sysMutex.Lock()
	defer fake.sysMutex.Unlock()
	fake.SysStub = stub
}

func (fake *FileInfo) SysReturns(result1 any) {
	fake.sysMutex.Lock()
	defer fake.sysMutex.Unlock()
	fake.SysStub = nil
	fake.sysReturns = struct {
		result1 any
	}{result1}
}

func (fake *FileInfo) SysReturnsOnCall(i int, result1 any) {
	fake.sysMutex.Lock()
	defer fake.sysMutex.Unlock()
	fake.SysStub = nil
	if fake.sysReturnsOnCall == nil {
		fake.sysReturnsOnCall = make(map[int]struct {
			result1 any
		})
	}
	fake.sysReturnsOnCall[i] = struct {
		result1 any
	}{result1}
}

func (fake *FileInfo) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.isDirMutex.RLock()
	defer fake.isDirMutex.RUnlock()
	fake.modTimeMutex.RLock()
	defer fake.modTimeMutex.RUnlock()
	fake.modeMutex.RLock()
	defer fake.modeMutex.RUnlock()
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	fake.sizeMutex.RLock()
	defer fake.sizeMutex.RUnlock()
	fake.sysMutex.RLock()
	defer fake.sysMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FileInfo) recordInvocation(key string, args []interface{}) {
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

var _ os.FileInfo = new(FileInfo)
