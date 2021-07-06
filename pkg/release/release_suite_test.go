package release_test

import (
	"testing"

	"github.com/matt-royal/biloba"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRelease(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "Release Suite", biloba.DefaultReporters())
}
