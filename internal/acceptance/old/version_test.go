package acceptance_test

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("version command", func() {
	Context("when given the version command", func() {
		It("prints the version number", func() {
			command := exec.Command(pathToMain, "version")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(fmt.Sprintf("kiln version %s", buildVersion)))
		})
	})

	Context("when given the -v short flag", func() {
		It("returns the binary version", func() {
			command := exec.Command(pathToMain, "-v")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, time.Second*10).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(fmt.Sprintf("kiln version %s", buildVersion)))
		})
	})

	Context("when given the --version long flag", func() {
		It("returns the binary version", func() {
			command := exec.Command(pathToMain, "--version")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, time.Second*10).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring(fmt.Sprintf("kiln version %s", buildVersion)))
		})
	})
})
