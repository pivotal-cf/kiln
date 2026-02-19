package carvel

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBaker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Carvel Baker Suite")
}
