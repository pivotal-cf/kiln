package fetcher_test

import (
	"testing"

	"github.com/matt-royal/biloba"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFetcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "Fetcher Suite", biloba.DefaultReporters())
}
