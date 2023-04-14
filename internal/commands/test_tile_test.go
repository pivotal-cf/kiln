package commands_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/stdcopy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
)

func init() {
	format.MaxLength = 100000
}

var _ = Describe("kiln test docker", func() {
	var (
		helloTileDirectorySegments []string
	)
	BeforeEach(func() {
		helloTileDirectorySegments = []string{"testdata", "test_tile", "hello-tile"}
		Expect(goVendor(filepath.Join(helloTileDirectorySegments...))).NotTo(HaveOccurred())
	})

	Context("locally missing docker image is built", func() {
		var (
			ctx    context.Context
			writer strings.Builder
			logger *log.Logger
		)

		BeforeEach(func() {
			ctx = context.Background()
			logger = log.New(&writer, "", 0)
		})

		Describe("successful creation creation", func() {
			var (
				fakeSshProvider *fakes.SshProvider
				helloTilePath   string
			)

			BeforeEach(func() {
				t := GinkgoT()
				helloTilePath = filepath.Join(helloTileDirectorySegments...)
				writePasswordToStdIn(t)
				fakeSshProvider = setupFakeSSHProvider()
				addTASFixtures(t, helloTilePath)
			})

			When("manifest tests should be successful", func() {
				const (
					testSuccessLogLine = "manifest tests completed successfully"
				)
				var (
					fakeMobyClient *fakes.MobyClient
				)
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testSuccessLogLine, 0)
				})
				It("properly executes tests", func() {
					subjectUnderTest := commands.NewManifestTest(logger, ctx, fakeMobyClient, fakeSshProvider)

					err := subjectUnderTest.Execute([]string{"--tile-path", helloTilePath, "--ginkgo-manifest-flags", "-r -slowSpecThreshold 1"})
					Expect(err).To(BeNil())

					By("logging helpful messages", func() {
						logs := writer.String()
						By("logging container information", func() {
							Expect(logs).To(ContainSubstring("tagged dont_push_me_vmware_confidential:123"))
							Expect(logs).To(ContainSubstring("Building / restoring cached docker image"))
						})
						By("logging test lines", func() {
							Expect(logs).To(ContainSubstring("manifest tests completed successfully"))
						})
					})

					By("creating a test container", func() {
						Expect(fakeMobyClient.ContainerCreateCallCount()).To(Equal(1))
						_, config, _, _, _, _ := fakeMobyClient.ContainerCreateArgsForCall(0)
						By("configuring the metadata and product config when they exist", func() {
							Expect(config.Env).To(Equal([]string{
								"TAS_METADATA_PATH=/tas/hello-tile/test/manifest/fixtures/tas_metadata.yml",
								"TAS_CONFIG_FILE=/tas/hello-tile/test/manifest/fixtures/tas_config.yml",
							}))
						})
						By("executing the tests", func() {
							dockerCmd := fmt.Sprintf("cd /tas/%s/test/manifest && PRODUCT=%[1]s RENDERER=ops-manifest ginkgo -r -slowSpecThreshold 1", "hello-tile")
							Expect(config.Cmd).To(Equal(strslice.StrSlice{"/bin/bash", "-c", dockerCmd}))
						})
					})
				})
			})

			When("manifest tests should be successful", func() {
				const (
					testFailureMessage = "exit status 1"
				)
				var (
					fakeMobyClient *fakes.MobyClient
				)
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testFailureMessage, 1)
				})
				It("returns an error", func() {
					subjectUnderTest := commands.NewManifestTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					err := subjectUnderTest.Execute([]string{"--tile-path", helloTilePath, "--ginkgo-manifest-flags", "-r -slowSpecThreshold 1"})
					Expect(err).To(HaveOccurred())

					By("logging helpful messages", func() {
						logs := writer.String()
						By("logging test lines", func() {
							Expect(logs).To(ContainSubstring("exit status 1"))
						})
					})
				})
			})
		})

		It("exits with an error if docker isn't running", func() {
			fakeMobyClient := &fakes.MobyClient{}
			fakeMobyClient.PingReturns(types.Ping{}, errors.New("docker not running"))
			fakeSshThinger := fakes.SshProvider{}
			fakeSshThinger.NeedsKeysReturns(false, nil)
			subjectUnderTest := commands.NewManifestTest(logger, ctx, fakeMobyClient, &fakeSshThinger)
			err := subjectUnderTest.Execute([]string{filepath.Join(helloTileDirectorySegments...)})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Docker daemon is not running"))
		})
	})
})

func setupFakeMobyClient(containerLogMessage string, testExitCode int64) *fakes.MobyClient {
	fakeMobyClient := &fakes.MobyClient{}
	fakeMobyClient.PingReturns(types.Ping{}, nil)
	r, w := io.Pipe()
	_ = w.Close()
	fakeConn := &fakes.Conn{R: r, W: stdcopy.NewStdWriter(io.Discard, stdcopy.Stdout)}
	fakeMobyClient.DialHijackReturns(fakeConn, nil)

	rc := io.NopCloser(strings.NewReader(`{"error": "", "message": "tagged dont_push_me_vmware_confidential:123"}`))
	imageBuildResponse := types.ImageBuildResponse{
		Body: rc,
	}
	fakeMobyClient.ImageBuildReturns(imageBuildResponse, nil)
	createResp := container.CreateResponse{
		ID: "some id",
	}
	fakeMobyClient.ContainerCreateReturns(createResp, nil)
	responses := make(chan container.WaitResponse)
	go func() {
		responses <- container.WaitResponse{
			Error:      nil,
			StatusCode: testExitCode,
		}
	}()
	fakeMobyClient.ContainerWaitReturns(responses, nil)
	rcLog := io.NopCloser(strings.NewReader(fmt.Sprintf(`{"error": "", "message": %q}"`, containerLogMessage)))
	fakeMobyClient.ContainerLogsReturns(rcLog, nil)
	fakeMobyClient.ContainerStartReturns(nil)
	return fakeMobyClient
}

type testingT interface {
	Helper()
	Cleanup(func())
	TempDir() string
	Fatal(args ...interface{})
	Name() string
}

func writePasswordToStdIn(t testingT) {
	t.Helper()
	oldStdin := os.Stdin
	t.Cleanup(func() {
		os.Stdin = oldStdin
	})
	passwd := "password\n"
	content := []byte(passwd)
	temporaryFile, err := os.CreateTemp(t.TempDir(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	_, err = temporaryFile.Write(content)
	if err != nil {
		t.Fatal(err)
	}
	_, err = temporaryFile.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = temporaryFile
}

func setupFakeSSHProvider() *fakes.SshProvider {
	fakeSSHProvider := fakes.SshProvider{}
	fakeSSHProvider.NeedsKeysReturns(false, nil)
	return &fakeSSHProvider
}

func addTASFixtures(t testingT, tileDirectory string) {
	fixturesDirectory := filepath.Join(tileDirectory, "test", "manifest", "fixtures")
	if err := os.MkdirAll(fixturesDirectory, 0766); err != nil {
		t.Fatal(err)
	}
	for _, filePath := range []string{
		filepath.Join(fixturesDirectory, "tas_metadata.yml"),
		filepath.Join(fixturesDirectory, "tas_config.yml"),
	} {
		if err := createEmptyFile(filePath); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.Remove(filePath)
		})
	}
}

func createEmptyFile(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	return nil
}
