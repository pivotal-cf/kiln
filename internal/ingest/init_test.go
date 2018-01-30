package ingest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIngest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/ingest")
}
