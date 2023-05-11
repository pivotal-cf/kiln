package commands_test

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
)

var _ = Describe("test", func() {
	Context("all tests succeed", func() {
		It("succeeds", func() {
			var testOutput bytes.Buffer
			logger := log.New(&testOutput, "", 0)
			ctx := context.Background()
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			Expect(err).NotTo(HaveOccurred())

			sshProvider, err := commands.NewSshProvider(commands.SSHClientCreator{})
			Expect(err).NotTo(HaveOccurred())
			tilePath := filepath.Join("testdata", "tas_fake", "tas")
			Expect(goVendor(tilePath)).NotTo(HaveOccurred())
			testTile := commands.NewTileTest(logger, ctx, cli, sshProvider)
			err = testTile.Execute([]string{"--verbose", "--tile-path", tilePath})

			Expect(err).NotTo(HaveOccurred())
			Expect(testOutput.String()).To(ContainSubstring("SUCCESS"))
			Expect(testOutput.String()).To(ContainSubstring("hello, world"))
			Expect(testOutput.String()).NotTo(ContainSubstring("Failure"))
		})
	})

	Context("all tests fail", func() {
		It("fails", func() {
			var testOutput bytes.Buffer
			logger := log.New(&testOutput, "", 0)
			ctx := context.Background()
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			Expect(err).NotTo(HaveOccurred())

			sshProvider, err := commands.NewSshProvider(commands.SSHClientCreator{})
			Expect(err).NotTo(HaveOccurred())
			testTile := commands.NewTileTest(logger, ctx, cli, sshProvider)
			tilePath := filepath.Join("testdata", "tas_fake", "tas_failing")
			Expect(goVendor(tilePath)).NotTo(HaveOccurred())
			err = testTile.Execute([]string{"--verbose", "--tile-path", tilePath})

			Expect(err).To(HaveOccurred())
			Expect(testOutput.String()).NotTo(ContainSubstring("SUCCESS"))
			Expect(testOutput.String()).To(ContainSubstring("Failure"))
		})
	})
})

func goVendor(modulePath string) error {
	var buf bytes.Buffer
	cmd := exec.Command("go", "mod", "vendor")
	cmd.Dir = modulePath
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		_, _ = os.Stderr.Write(buf.Bytes())
	}
	return err
}
