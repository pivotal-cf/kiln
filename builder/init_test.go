package builder_test

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

//go:generate counterfeiter -o ./fakes/read_closer.go --fake-name ReadCloser io.ReadCloser

func TestBuilder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "builder")
}

func NewBuffer(b *bytes.Buffer) *Buffer {
	return &Buffer{
		buffer: b,
	}
}

type Buffer struct {
	buffer *bytes.Buffer
	Error  error
}

func (b *Buffer) Read(p []byte) (int, error) {
	if b.Error != nil {
		return 0, b.Error
	}

	return b.buffer.Read(p)
}

func (b *Buffer) Write(p []byte) (int, error) {
	return b.buffer.Write(p)
}

func (b Buffer) Close() error {
	return nil
}
