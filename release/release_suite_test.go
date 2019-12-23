package release_test

import (
	"github.com/matt-royal/biloba"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRelease(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "Release Suite", biloba.DefaultReporters())
}
