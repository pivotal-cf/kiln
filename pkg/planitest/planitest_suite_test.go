package planitest_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Planitest Suite")
}
