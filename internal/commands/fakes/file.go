// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/pivotal-cf/kiln/internal/commands"
)

type File struct {
	CloseStub        func() error
	closeMutex       sync.RWMutex
	closeArgsForCall []struct {
	}
	closeReturns struct {
		result1 error
	}
	closeReturnsOnCall map[int]struct {
		result1 error
	}
	LockStub        func() error
	lockMutex       sync.RWMutex
	lockArgsForCall []struct {
	}
	lockReturns struct {
		result1 error
	}
	lockReturnsOnCall map[int]struct {
		result1 error
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
	ReadStub        func([]byte) (int, error)
	readMutex       sync.RWMutex
	readArgsForCall []struct {
		arg1 []byte
	}
	readReturns struct {
		result1 int
		result2 error
	}
	readReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	ReadAtStub        func([]byte, int64) (int, error)
	readAtMutex       sync.RWMutex
	readAtArgsForCall []struct {
		arg1 []byte
		arg2 int64
	}
	readAtReturns struct {
		result1 int
		result2 error
	}
	readAtReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	SeekStub        func(int64, int) (int64, error)
	seekMutex       sync.RWMutex
	seekArgsForCall []struct {
		arg1 int64
		arg2 int
	}
	seekReturns struct {
		result1 int64
		result2 error
	}
	seekReturnsOnCall map[int]struct {
		result1 int64
		result2 error
	}
	TruncateStub        func(int64) error
	truncateMutex       sync.RWMutex
	truncateArgsForCall []struct {
		arg1 int64
	}
	truncateReturns struct {
		result1 error
	}
	truncateReturnsOnCall map[int]struct {
		result1 error
	}
	UnlockStub        func() error
	unlockMutex       sync.RWMutex
	unlockArgsForCall []struct {
	}
	unlockReturns struct {
		result1 error
	}
	unlockReturnsOnCall map[int]struct {
		result1 error
	}
	WriteStub        func([]byte) (int, error)
	writeMutex       sync.RWMutex
	writeArgsForCall []struct {
		arg1 []byte
	}
	writeReturns struct {
		result1 int
		result2 error
	}
	writeReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *File) Close() error {
	fake.closeMutex.Lock()
	ret, specificReturn := fake.closeReturnsOnCall[len(fake.closeArgsForCall)]
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct {
	}{})
	stub := fake.CloseStub
	fakeReturns := fake.closeReturns
	fake.recordInvocation("Close", []interface{}{})
	fake.closeMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *File) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *File) CloseCalls(stub func() error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = stub
}

func (fake *File) CloseReturns(result1 error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = nil
	fake.closeReturns = struct {
		result1 error
	}{result1}
}

func (fake *File) CloseReturnsOnCall(i int, result1 error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = nil
	if fake.closeReturnsOnCall == nil {
		fake.closeReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.closeReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *File) Lock() error {
	fake.lockMutex.Lock()
	ret, specificReturn := fake.lockReturnsOnCall[len(fake.lockArgsForCall)]
	fake.lockArgsForCall = append(fake.lockArgsForCall, struct {
	}{})
	stub := fake.LockStub
	fakeReturns := fake.lockReturns
	fake.recordInvocation("Lock", []interface{}{})
	fake.lockMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *File) LockCallCount() int {
	fake.lockMutex.RLock()
	defer fake.lockMutex.RUnlock()
	return len(fake.lockArgsForCall)
}

func (fake *File) LockCalls(stub func() error) {
	fake.lockMutex.Lock()
	defer fake.lockMutex.Unlock()
	fake.LockStub = stub
}

func (fake *File) LockReturns(result1 error) {
	fake.lockMutex.Lock()
	defer fake.lockMutex.Unlock()
	fake.LockStub = nil
	fake.lockReturns = struct {
		result1 error
	}{result1}
}

func (fake *File) LockReturnsOnCall(i int, result1 error) {
	fake.lockMutex.Lock()
	defer fake.lockMutex.Unlock()
	fake.LockStub = nil
	if fake.lockReturnsOnCall == nil {
		fake.lockReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.lockReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *File) Name() string {
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

func (fake *File) NameCallCount() int {
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	return len(fake.nameArgsForCall)
}

func (fake *File) NameCalls(stub func() string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = stub
}

func (fake *File) NameReturns(result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	fake.nameReturns = struct {
		result1 string
	}{result1}
}

func (fake *File) NameReturnsOnCall(i int, result1 string) {
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

func (fake *File) Read(arg1 []byte) (int, error) {
	var arg1Copy []byte
	if arg1 != nil {
		arg1Copy = make([]byte, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.readMutex.Lock()
	ret, specificReturn := fake.readReturnsOnCall[len(fake.readArgsForCall)]
	fake.readArgsForCall = append(fake.readArgsForCall, struct {
		arg1 []byte
	}{arg1Copy})
	stub := fake.ReadStub
	fakeReturns := fake.readReturns
	fake.recordInvocation("Read", []interface{}{arg1Copy})
	fake.readMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *File) ReadCallCount() int {
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	return len(fake.readArgsForCall)
}

func (fake *File) ReadCalls(stub func([]byte) (int, error)) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = stub
}

func (fake *File) ReadArgsForCall(i int) []byte {
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	argsForCall := fake.readArgsForCall[i]
	return argsForCall.arg1
}

func (fake *File) ReadReturns(result1 int, result2 error) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = nil
	fake.readReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *File) ReadReturnsOnCall(i int, result1 int, result2 error) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = nil
	if fake.readReturnsOnCall == nil {
		fake.readReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.readReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *File) ReadAt(arg1 []byte, arg2 int64) (int, error) {
	var arg1Copy []byte
	if arg1 != nil {
		arg1Copy = make([]byte, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.readAtMutex.Lock()
	ret, specificReturn := fake.readAtReturnsOnCall[len(fake.readAtArgsForCall)]
	fake.readAtArgsForCall = append(fake.readAtArgsForCall, struct {
		arg1 []byte
		arg2 int64
	}{arg1Copy, arg2})
	stub := fake.ReadAtStub
	fakeReturns := fake.readAtReturns
	fake.recordInvocation("ReadAt", []interface{}{arg1Copy, arg2})
	fake.readAtMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *File) ReadAtCallCount() int {
	fake.readAtMutex.RLock()
	defer fake.readAtMutex.RUnlock()
	return len(fake.readAtArgsForCall)
}

func (fake *File) ReadAtCalls(stub func([]byte, int64) (int, error)) {
	fake.readAtMutex.Lock()
	defer fake.readAtMutex.Unlock()
	fake.ReadAtStub = stub
}

func (fake *File) ReadAtArgsForCall(i int) ([]byte, int64) {
	fake.readAtMutex.RLock()
	defer fake.readAtMutex.RUnlock()
	argsForCall := fake.readAtArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *File) ReadAtReturns(result1 int, result2 error) {
	fake.readAtMutex.Lock()
	defer fake.readAtMutex.Unlock()
	fake.ReadAtStub = nil
	fake.readAtReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *File) ReadAtReturnsOnCall(i int, result1 int, result2 error) {
	fake.readAtMutex.Lock()
	defer fake.readAtMutex.Unlock()
	fake.ReadAtStub = nil
	if fake.readAtReturnsOnCall == nil {
		fake.readAtReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.readAtReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *File) Seek(arg1 int64, arg2 int) (int64, error) {
	fake.seekMutex.Lock()
	ret, specificReturn := fake.seekReturnsOnCall[len(fake.seekArgsForCall)]
	fake.seekArgsForCall = append(fake.seekArgsForCall, struct {
		arg1 int64
		arg2 int
	}{arg1, arg2})
	stub := fake.SeekStub
	fakeReturns := fake.seekReturns
	fake.recordInvocation("Seek", []interface{}{arg1, arg2})
	fake.seekMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *File) SeekCallCount() int {
	fake.seekMutex.RLock()
	defer fake.seekMutex.RUnlock()
	return len(fake.seekArgsForCall)
}

func (fake *File) SeekCalls(stub func(int64, int) (int64, error)) {
	fake.seekMutex.Lock()
	defer fake.seekMutex.Unlock()
	fake.SeekStub = stub
}

func (fake *File) SeekArgsForCall(i int) (int64, int) {
	fake.seekMutex.RLock()
	defer fake.seekMutex.RUnlock()
	argsForCall := fake.seekArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *File) SeekReturns(result1 int64, result2 error) {
	fake.seekMutex.Lock()
	defer fake.seekMutex.Unlock()
	fake.SeekStub = nil
	fake.seekReturns = struct {
		result1 int64
		result2 error
	}{result1, result2}
}

func (fake *File) SeekReturnsOnCall(i int, result1 int64, result2 error) {
	fake.seekMutex.Lock()
	defer fake.seekMutex.Unlock()
	fake.SeekStub = nil
	if fake.seekReturnsOnCall == nil {
		fake.seekReturnsOnCall = make(map[int]struct {
			result1 int64
			result2 error
		})
	}
	fake.seekReturnsOnCall[i] = struct {
		result1 int64
		result2 error
	}{result1, result2}
}

func (fake *File) Truncate(arg1 int64) error {
	fake.truncateMutex.Lock()
	ret, specificReturn := fake.truncateReturnsOnCall[len(fake.truncateArgsForCall)]
	fake.truncateArgsForCall = append(fake.truncateArgsForCall, struct {
		arg1 int64
	}{arg1})
	stub := fake.TruncateStub
	fakeReturns := fake.truncateReturns
	fake.recordInvocation("Truncate", []interface{}{arg1})
	fake.truncateMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *File) TruncateCallCount() int {
	fake.truncateMutex.RLock()
	defer fake.truncateMutex.RUnlock()
	return len(fake.truncateArgsForCall)
}

func (fake *File) TruncateCalls(stub func(int64) error) {
	fake.truncateMutex.Lock()
	defer fake.truncateMutex.Unlock()
	fake.TruncateStub = stub
}

func (fake *File) TruncateArgsForCall(i int) int64 {
	fake.truncateMutex.RLock()
	defer fake.truncateMutex.RUnlock()
	argsForCall := fake.truncateArgsForCall[i]
	return argsForCall.arg1
}

func (fake *File) TruncateReturns(result1 error) {
	fake.truncateMutex.Lock()
	defer fake.truncateMutex.Unlock()
	fake.TruncateStub = nil
	fake.truncateReturns = struct {
		result1 error
	}{result1}
}

func (fake *File) TruncateReturnsOnCall(i int, result1 error) {
	fake.truncateMutex.Lock()
	defer fake.truncateMutex.Unlock()
	fake.TruncateStub = nil
	if fake.truncateReturnsOnCall == nil {
		fake.truncateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.truncateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *File) Unlock() error {
	fake.unlockMutex.Lock()
	ret, specificReturn := fake.unlockReturnsOnCall[len(fake.unlockArgsForCall)]
	fake.unlockArgsForCall = append(fake.unlockArgsForCall, struct {
	}{})
	stub := fake.UnlockStub
	fakeReturns := fake.unlockReturns
	fake.recordInvocation("Unlock", []interface{}{})
	fake.unlockMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *File) UnlockCallCount() int {
	fake.unlockMutex.RLock()
	defer fake.unlockMutex.RUnlock()
	return len(fake.unlockArgsForCall)
}

func (fake *File) UnlockCalls(stub func() error) {
	fake.unlockMutex.Lock()
	defer fake.unlockMutex.Unlock()
	fake.UnlockStub = stub
}

func (fake *File) UnlockReturns(result1 error) {
	fake.unlockMutex.Lock()
	defer fake.unlockMutex.Unlock()
	fake.UnlockStub = nil
	fake.unlockReturns = struct {
		result1 error
	}{result1}
}

func (fake *File) UnlockReturnsOnCall(i int, result1 error) {
	fake.unlockMutex.Lock()
	defer fake.unlockMutex.Unlock()
	fake.UnlockStub = nil
	if fake.unlockReturnsOnCall == nil {
		fake.unlockReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.unlockReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *File) Write(arg1 []byte) (int, error) {
	var arg1Copy []byte
	if arg1 != nil {
		arg1Copy = make([]byte, len(arg1))
		copy(arg1Copy, arg1)
	}
	fake.writeMutex.Lock()
	ret, specificReturn := fake.writeReturnsOnCall[len(fake.writeArgsForCall)]
	fake.writeArgsForCall = append(fake.writeArgsForCall, struct {
		arg1 []byte
	}{arg1Copy})
	stub := fake.WriteStub
	fakeReturns := fake.writeReturns
	fake.recordInvocation("Write", []interface{}{arg1Copy})
	fake.writeMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *File) WriteCallCount() int {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	return len(fake.writeArgsForCall)
}

func (fake *File) WriteCalls(stub func([]byte) (int, error)) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = stub
}

func (fake *File) WriteArgsForCall(i int) []byte {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	argsForCall := fake.writeArgsForCall[i]
	return argsForCall.arg1
}

func (fake *File) WriteReturns(result1 int, result2 error) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = nil
	fake.writeReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *File) WriteReturnsOnCall(i int, result1 int, result2 error) {
	fake.writeMutex.Lock()
	defer fake.writeMutex.Unlock()
	fake.WriteStub = nil
	if fake.writeReturnsOnCall == nil {
		fake.writeReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.writeReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *File) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	fake.lockMutex.RLock()
	defer fake.lockMutex.RUnlock()
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	fake.readAtMutex.RLock()
	defer fake.readAtMutex.RUnlock()
	fake.seekMutex.RLock()
	defer fake.seekMutex.RUnlock()
	fake.truncateMutex.RLock()
	defer fake.truncateMutex.RUnlock()
	fake.unlockMutex.RLock()
	defer fake.unlockMutex.RUnlock()
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *File) recordInvocation(key string, args []interface{}) {
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

var _ commands.File = new(File)
