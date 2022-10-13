package proofing_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProofing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "proofing")
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
