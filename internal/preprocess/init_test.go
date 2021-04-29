package preprocess_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPreprocessMetadata(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "preprocess-metadata")
}
