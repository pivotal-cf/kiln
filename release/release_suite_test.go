package release_test

import (
	"github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"testing"
)

var suite spec.Suite

func init() {
	suite = spec.New("release")
	suite.Before(func(t *testing.T) {
		gomega.RegisterTestingT(t)
	})
	suite("builtRelease", testBuiltRelease)
	suite("compiledRelease", testCompiledRelease)
}

func TestRelease(t *testing.T) {
	suite.Run(t)
}
