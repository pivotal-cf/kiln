package acceptance_test

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var pathToMain string

func TestCarvelAcceptance(t *testing.T) {
	SetDefaultEventuallyTimeout(time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "carvel acceptance")
}

var _ = BeforeSuite(func() {
	if _, err := exec.LookPath("bosh"); err != nil {
		Skip("bosh CLI not installed - skipping carvel acceptance tests")
	}

	var err error
	pathToMain, err = gexec.Build("github.com/pivotal-cf/kiln")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = Describe("carvel bake command", func() {
	var (
		outputFile      string
		tmpDir          string
		inputPath       string
		commandWithArgs []string
	)

	const (
		sampleTileFixture = "fixtures/sample-tile"
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "kiln-carvel-test")
		Expect(err).NotTo(HaveOccurred())

		// Copy the sample-tile fixture to a temp directory
		inputPath = filepath.Join(tmpDir, "tile")
		err = os.CopyFS(inputPath, os.DirFS(sampleTileFixture))
		Expect(err).NotTo(HaveOccurred())

		// Initialize git repo (required by kiln for metadata)
		gitCommands := []*exec.Cmd{
			exec.Command("git", "init"),
			exec.Command("git", "config", "user.email", "test@test.com"),
			exec.Command("git", "config", "user.name", "Test"),
			exec.Command("git", "add", "."),
			exec.Command("git", "commit", "-m", "initial commit"),
		}
		for _, cmd := range gitCommands {
			cmd.Dir = inputPath
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "error invoking git: "+string(out))
		}

		outputFile = filepath.Join(tmpDir, "k8s-tile-test-0.0.1.pivotal")

		commandWithArgs = []string{
			"carvel", "bake",
			"--source-directory", inputPath,
			"--output-file", outputFile,
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	It("generates a tile with the correct structure", func() {
		commandWithArgs = append(commandWithArgs, "--verbose")

		command := exec.Command(pathToMain, commandWithArgs...)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, "60s").Should(gexec.Exit(0))

		archive, err := os.Open(outputFile)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = archive.Close() }()

		archiveInfo, err := archive.Stat()
		Expect(err).NotTo(HaveOccurred())

		bakedTile, err := zip.NewReader(archive, archiveInfo.Size())
		Expect(err).NotTo(HaveOccurred())

		// Verify metadata exists
		_, err = bakedTile.Open("metadata/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		// Verify releases directory contains a tgz
		var foundRelease bool
		for _, f := range bakedTile.File {
			if filepath.Dir(f.Name) == "releases" && filepath.Ext(f.Name) == ".tgz" {
				foundRelease = true
				break
			}
		}
		Expect(foundRelease).To(BeTrue(), "releases/*.tgz should be in the tile")

		// Verify migrations directory exists (even if empty)
		var foundMigrations bool
		for _, f := range bakedTile.File {
			if filepath.Dir(f.Name) == "migrations" || f.Name == "migrations/v1/" {
				foundMigrations = true
				break
			}
		}
		Expect(foundMigrations).To(BeTrue(), "migrations directory should be in the tile")

		Eventually(session.Out).Should(gbytes.Say("Baked"))
		Eventually(session.Out).Should(gbytes.Say("k8s-tile-test"))
	})

	It("produces a valid zip archive", func() {
		command := exec.Command(pathToMain, commandWithArgs...)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, "60s").Should(gexec.Exit(0))

		// Verify using unzip -t
		verifyCmd := exec.Command("unzip", "-t", outputFile)
		verifySession, err := gexec.Start(verifyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(verifySession, "10s").Should(gexec.Exit(0))
		Eventually(verifySession.Out).Should(gbytes.Say("No errors detected"))
	})

	Context("failure cases", func() {
		Context("when the output-file flag is not provided", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain, "carvel", "bake",
					"--source-directory", inputPath,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Eventually(session.Err).Should(gbytes.Say("output-file"))
			})
		})

		Context("when the source directory does not exist", func() {
			It("prints an error and exits 1", func() {
				command := exec.Command(pathToMain, "carvel", "bake",
					"--source-directory", "/non/existent/path",
					"--output-file", outputFile,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
			})
		})

		Context("when the source directory is missing base.yml", func() {
			It("prints an error and exits 1", func() {
				emptyDir, err := os.MkdirTemp(tmpDir, "empty-tile")
				Expect(err).NotTo(HaveOccurred())

				command := exec.Command(pathToMain, "carvel", "bake",
					"--source-directory", emptyDir,
					"--output-file", outputFile,
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(1))
				Eventually(session.Err).Should(gbytes.Say("base.yml"))
			})
		})
	})
})
