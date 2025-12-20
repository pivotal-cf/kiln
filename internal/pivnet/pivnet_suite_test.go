package pivnet_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPivnet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pivnet Suite")
}
