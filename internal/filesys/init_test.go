package filesys_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func TestFileSys(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helper")
}
