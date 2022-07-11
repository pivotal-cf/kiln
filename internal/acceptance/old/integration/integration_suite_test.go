package integration_test

import (
	"os"
	"testing"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var pathToMain string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance/integration")
}

var _ = BeforeSuite(func() {
	var err error
	pathToMain, err = gexec.Build("github.com/pivotal-cf/kiln")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=\"true\" to run them")
	}
})
