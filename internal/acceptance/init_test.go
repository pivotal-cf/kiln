package acceptance_test

import (
	"github.com/matt-royal/biloba"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var pathToMain string

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "acceptance", biloba.DefaultReporters())
}

var _ = BeforeSuite(func() {
	var err error
	pathToMain, err = gexec.Build("github.com/pivotal-cf/kiln")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
