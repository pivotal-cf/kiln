package fakes

import (
	"context"
	"sync"
)

type TrainstatClient struct {
	FetchStub          func(milestone string, version string, tile string) ([]string, error)
	fetchMutex         sync.RWMutex
	fetchReturnsOnCall map[int]struct {
		result1 []string
		result2 error
	}
	fetchArgsForCall []struct {
		ctx  context.Context
		arg1 string
		arg2 string
		arg3 string
	}
	fetchReturns struct {
		result1 []string
		result2 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *TrainstatClient) FetchTrainstatNotes(ctx context.Context, arg1 string, arg2 string, arg3 string) (notes []string, err error) {
	fake.fetchMutex.Lock()
	ret, specificReturn := fake.fetchReturnsOnCall[len(fake.fetchArgsForCall)]
	fake.fetchArgsForCall = append(fake.fetchArgsForCall, struct {
		ctx  context.Context
		arg1 string
		arg2 string
		arg3 string
	}{ctx, arg1, arg2, arg3})
	stub := fake.FetchStub
	fakeReturns := fake.fetchReturns
	fake.recordInvocation("Get", []any{arg1, arg2, arg3})
	fake.fetchMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *TrainstatClient) FetchReturnsOnCall(i int, result1 []string, result2 error) {
	fake.fetchMutex.Lock()
	defer fake.fetchMutex.Unlock()
	fake.FetchStub = nil
	if fake.fetchReturnsOnCall == nil {
		fake.fetchReturnsOnCall = make(map[int]struct {
			result1 []string
			result2 error
		})
	}
	fake.fetchReturnsOnCall[i] = struct {
		result1 []string
		result2 error
	}{result1, result2}
}

func (fake *TrainstatClient) GetCallCount() int {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return len(fake.fetchArgsForCall)
}

func (fake *TrainstatClient) GetCalls(stub func(string, string, string) ([]string, error)) {
	fake.fetchMutex.Lock()
	defer fake.fetchMutex.Unlock()
	fake.FetchStub = stub
}

func (fake *TrainstatClient) GetArgsForCall(i int) (string, string, string) {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	argsForCall := fake.fetchArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *TrainstatClient) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *TrainstatClient) recordInvocation(key string, args []any) {
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
