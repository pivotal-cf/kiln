package commands

import (
	"bytes"
	"log"

	"github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("test", func() {
	Context("manifest tests succeed", func() {
		It("succeeds", func() {
			var testOutput bytes.Buffer
			logger := log.New(&testOutput, "", 0)
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			Expect(err).NotTo(HaveOccurred())

			sshProvider, err := NewSSHProvider(SSHClientConstructor{})
			Expect(err).NotTo(HaveOccurred())
			testTile := NewManifestTest(logger, cli, sshProvider)
			err = testTile.Execute([]string{"--tile-path", "tas_fake/tas"})

			Expect(err).NotTo(HaveOccurred())
			Expect(testOutput.String()).To(ContainSubstring("SUCCESS"))
			Expect(testOutput.String()).NotTo(ContainSubstring("Failure"))
		})
	})

	Context("manifest tests fail", func() {
		It("fails", func() {
			var testOutput bytes.Buffer
			logger := log.New(&testOutput, "", 0)
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			Expect(err).NotTo(HaveOccurred())

			sshProvider, err := NewSSHProvider(SSHClientConstructor{})
			Expect(err).NotTo(HaveOccurred())
			testTile := NewManifestTest(logger, cli, sshProvider)
			err = testTile.Execute([]string{"--tile-path", "tas_fake/tas_failing"})

			Expect(err).To(HaveOccurred())
			Expect(testOutput.String()).NotTo(ContainSubstring("SUCCESS"))
			Expect(testOutput.String()).To(ContainSubstring("Failure"))
		})
	})
})
