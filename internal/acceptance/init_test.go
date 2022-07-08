package acceptance_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	pathToMain   string
	buildVersion string
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance")
}

var _ = BeforeSuite(func() {
	buildVersion = fmt.Sprintf("v0.0.0-dev.%d", time.Now().Unix())

	var err error
	pathToMain, err = gexec.Build("github.com/pivotal-cf/kiln",
		"--ldflags", fmt.Sprintf("-X main.version=%s", buildVersion))
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
