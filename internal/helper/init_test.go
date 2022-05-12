package helper_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHelper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helper")
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
