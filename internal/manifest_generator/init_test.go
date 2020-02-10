package manifest_generator_test

import (
	"github.com/matt-royal/biloba"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHelper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "manifest_generator", biloba.DefaultReporters())
}
