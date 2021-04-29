package proofing_test

import (
	"testing"

	"github.com/matt-royal/biloba"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProofing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "proofing", biloba.DefaultReporters())
}
