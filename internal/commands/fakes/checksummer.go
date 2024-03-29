// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"
)

type Checksummer struct {
	SumStub        func(string) error
	sumMutex       sync.RWMutex
	sumArgsForCall []struct {
		arg1 string
	}
	sumReturns struct {
		result1 error
	}
	sumReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *Checksummer) Sum(arg1 string) error {
	fake.sumMutex.Lock()
	ret, specificReturn := fake.sumReturnsOnCall[len(fake.sumArgsForCall)]
	fake.sumArgsForCall = append(fake.sumArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.SumStub
	fakeReturns := fake.sumReturns
	fake.recordInvocation("Sum", []interface{}{arg1})
	fake.sumMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *Checksummer) SumCallCount() int {
	fake.sumMutex.RLock()
	defer fake.sumMutex.RUnlock()
	return len(fake.sumArgsForCall)
}

func (fake *Checksummer) SumCalls(stub func(string) error) {
	fake.sumMutex.Lock()
	defer fake.sumMutex.Unlock()
	fake.SumStub = stub
}

func (fake *Checksummer) SumArgsForCall(i int) string {
	fake.sumMutex.RLock()
	defer fake.sumMutex.RUnlock()
	argsForCall := fake.sumArgsForCall[i]
	return argsForCall.arg1
}

func (fake *Checksummer) SumReturns(result1 error) {
	fake.sumMutex.Lock()
	defer fake.sumMutex.Unlock()
	fake.SumStub = nil
	fake.sumReturns = struct {
		result1 error
	}{result1}
}

func (fake *Checksummer) SumReturnsOnCall(i int, result1 error) {
	fake.sumMutex.Lock()
	defer fake.sumMutex.Unlock()
	fake.SumStub = nil
	if fake.sumReturnsOnCall == nil {
		fake.sumReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.sumReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *Checksummer) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.sumMutex.RLock()
	defer fake.sumMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *Checksummer) recordInvocation(key string, args []interface{}) {
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
