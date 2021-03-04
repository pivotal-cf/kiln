package preprocess_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestPreprocessMetadata(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "preprocess-metadata")
}
