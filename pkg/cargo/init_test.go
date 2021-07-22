package cargo_test

import (
	"testing"

	"github.com/matt-royal/biloba"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCargo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "internal/cargo", biloba.GoLandReporter())
}
