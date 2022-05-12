package baking_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func TestBaking(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/baking")
}
