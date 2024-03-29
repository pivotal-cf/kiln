// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"io"
	"sync"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/kiln/internal/component"
)

type S3Downloader struct {
	DownloadStub        func(io.WriterAt, *s3.GetObjectInput, ...func(*s3manager.Downloader)) (int64, error)
	downloadMutex       sync.RWMutex
	downloadArgsForCall []struct {
		arg1 io.WriterAt
		arg2 *s3.GetObjectInput
		arg3 []func(*s3manager.Downloader)
	}
	downloadReturns struct {
		result1 int64
		result2 error
	}
	downloadReturnsOnCall map[int]struct {
		result1 int64
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *S3Downloader) Download(arg1 io.WriterAt, arg2 *s3.GetObjectInput, arg3 ...func(*s3manager.Downloader)) (int64, error) {
	fake.downloadMutex.Lock()
	ret, specificReturn := fake.downloadReturnsOnCall[len(fake.downloadArgsForCall)]
	fake.downloadArgsForCall = append(fake.downloadArgsForCall, struct {
		arg1 io.WriterAt
		arg2 *s3.GetObjectInput
		arg3 []func(*s3manager.Downloader)
	}{arg1, arg2, arg3})
	stub := fake.DownloadStub
	fakeReturns := fake.downloadReturns
	fake.recordInvocation("Download", []interface{}{arg1, arg2, arg3})
	fake.downloadMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *S3Downloader) DownloadCallCount() int {
	fake.downloadMutex.RLock()
	defer fake.downloadMutex.RUnlock()
	return len(fake.downloadArgsForCall)
}

func (fake *S3Downloader) DownloadCalls(stub func(io.WriterAt, *s3.GetObjectInput, ...func(*s3manager.Downloader)) (int64, error)) {
	fake.downloadMutex.Lock()
	defer fake.downloadMutex.Unlock()
	fake.DownloadStub = stub
}

func (fake *S3Downloader) DownloadArgsForCall(i int) (io.WriterAt, *s3.GetObjectInput, []func(*s3manager.Downloader)) {
	fake.downloadMutex.RLock()
	defer fake.downloadMutex.RUnlock()
	argsForCall := fake.downloadArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *S3Downloader) DownloadReturns(result1 int64, result2 error) {
	fake.downloadMutex.Lock()
	defer fake.downloadMutex.Unlock()
	fake.DownloadStub = nil
	fake.downloadReturns = struct {
		result1 int64
		result2 error
	}{result1, result2}
}

func (fake *S3Downloader) DownloadReturnsOnCall(i int, result1 int64, result2 error) {
	fake.downloadMutex.Lock()
	defer fake.downloadMutex.Unlock()
	fake.DownloadStub = nil
	if fake.downloadReturnsOnCall == nil {
		fake.downloadReturnsOnCall = make(map[int]struct {
			result1 int64
			result2 error
		})
	}
	fake.downloadReturnsOnCall[i] = struct {
		result1 int64
		result2 error
	}{result1, result2}
}

func (fake *S3Downloader) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.downloadMutex.RLock()
	defer fake.downloadMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *S3Downloader) recordInvocation(key string, args []interface{}) {
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

var _ component.S3Downloader = new(S3Downloader)
