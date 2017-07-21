package kiln

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var pathToMain string

func TestKiln(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kiln")
}
