package cargo_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCargo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/cargo")
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }
