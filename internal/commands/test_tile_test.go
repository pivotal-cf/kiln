package commands_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	var helloTileDirectorySegments []string
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

		Describe("test outcomes", func() {
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
				var fakeMobyClient *fakes.MobyClient
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testSuccessLogLine, 0)
				})
				When("executing tests", func() {
					var subjectUnderTest commands.TileTest
					BeforeEach(func() {
						writer.Reset()
						subjectUnderTest = commands.NewTileTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					})
					When("verbose is passed", func() {
						It("succeeds and logs info", func() {
							err := subjectUnderTest.Execute([]string{"--manifest", "--verbose", "--tile-path", helloTilePath, "--ginkgo-flags", "-r -slowSpecThreshold 1"})
							Expect(err).To(BeNil())

							By("logging helpful messages", func() {
								logs := writer.String()
								By("logging container information", func() {
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
									expected := []string{
										"PRODUCT=hello-tile",
										"RENDERER=ops-manifest",
										"TAS_METADATA_PATH=/tas/hello-tile/test/manifest/fixtures/tas_metadata.yml",
										"TAS_CONFIG_FILE=/tas/hello-tile/test/manifest/fixtures/tas_config.yml",
									}
									actual := config.Env
									sort.Strings(actual)
									sort.Strings(expected)

									Expect(actual).To(BeEquivalentTo(expected))
								})
								By("executing the tests", func() {
									expected := []string{
										"PRODUCT=hello-tile",
										"RENDERER=ops-manifest",
										"TAS_METADATA_PATH=/tas/hello-tile/test/manifest/fixtures/tas_metadata.yml",
										"TAS_CONFIG_FILE=/tas/hello-tile/test/manifest/fixtures/tas_config.yml",
									}
									actual := config.Env
									sort.Strings(actual)
									sort.Strings(expected)

									dockerCmd := "cd /tas/hello-tile && ginkgo -r -slowSpecThreshold 1 /tas/hello-tile/test/manifest"
									Expect(config.Cmd).To(Equal(strslice.StrSlice{"/bin/bash", "-c", dockerCmd}))
								})
							})
						})
					})
					When("verbose isn't passed", func() {
						It("doesn't log info", func() {
							err := subjectUnderTest.Execute([]string{"--manifest", "--tile-path", helloTilePath, "--ginkgo-flags", "-r -slowSpecThreshold 1"})
							Expect(err).To(BeNil())

							By("logging helpful messages", func() {
								logs := writer.String()
								By("logging container information", func() {
									Expect(logs).NotTo(ContainSubstring("Building / restoring cached docker image"))
									Expect(logs).NotTo(ContainSubstring("Info:"))
								})
								By("logging test lines", func() {
									Expect(logs).To(ContainSubstring("manifest tests completed successfully"))
								})
							})
						})
					})
				})
			})

			When("manifest tests shouldn't be successful", func() {
				const (
					testFailureMessage = "exit status 1"
				)
				var fakeMobyClient *fakes.MobyClient
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testFailureMessage, 1)
				})
				It("returns an error", func() {
					subjectUnderTest := commands.NewTileTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					err := subjectUnderTest.Execute([]string{"--manifest", "--verbose", "--tile-path", helloTilePath, "--ginkgo-flags", "-r -slowSpecThreshold 1"})
					Expect(err).To(HaveOccurred())

					By("logging helpful messages", func() {
						logs := writer.String()
						By("logging test lines", func() {
							Expect(logs).To(ContainSubstring("exit status 1"))
						})
					})
				})
			})

			When("stability tests should be successful", func() {
				const (
					testSuccessLogLine = "manifest tests completed successfully"
				)
				var fakeMobyClient *fakes.MobyClient
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testSuccessLogLine, 0)
				})
				When("executing tests", func() {
					var subjectUnderTest commands.TileTest
					BeforeEach(func() {
						writer.Reset()
						subjectUnderTest = commands.NewTileTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					})
					When("verbose is passed", func() {
						It("succeeds and logs info", func() {
							err := subjectUnderTest.Execute([]string{"--stability", "--verbose", "--tile-path", helloTilePath, "--ginkgo-flags", "-r -slowSpecThreshold 1"})
							Expect(err).To(BeNil())

							By("logging helpful messages", func() {
								logs := writer.String()
								By("logging container information", func() {
									Expect(logs).To(ContainSubstring("Building / restoring cached docker image"))
								})
								By("logging test lines", func() {
									Expect(logs).To(ContainSubstring("manifest tests completed successfully"))
								})
							})

							By("creating a test container", func() {
								Expect(fakeMobyClient.ContainerCreateCallCount()).To(Equal(1))
								_, config, _, _, _, _ := fakeMobyClient.ContainerCreateArgsForCall(0)

								By("executing the tests", func() {
									expected := []string{
										"PRODUCT=hello-tile",
										"RENDERER=ops-manifest",
										"TAS_METADATA_PATH=/tas/hello-tile/test/manifest/fixtures/tas_metadata.yml",
										"TAS_CONFIG_FILE=/tas/hello-tile/test/manifest/fixtures/tas_config.yml",
									}
									actual := config.Env
									sort.Strings(actual)
									sort.Strings(expected)

									dockerCmd := "cd /tas/hello-tile && ginkgo -r -slowSpecThreshold 1 /tas/hello-tile/test/stability"
									Expect(config.Cmd).To(Equal(strslice.StrSlice{"/bin/bash", "-c", dockerCmd}))
								})
							})
						})
					})
					When("verbose isn't passed", func() {
						It("doesn't log info", func() {
							err := subjectUnderTest.Execute([]string{"--stability", "--tile-path", helloTilePath, "--ginkgo-flags", "-r -slowSpecThreshold 1"})
							Expect(err).To(BeNil())

							By("logging helpful messages", func() {
								logs := writer.String()
								By("logging container information", func() {
									Expect(logs).NotTo(ContainSubstring("Building / restoring cached docker image"))
									Expect(logs).NotTo(ContainSubstring("Info:"))
								})
								By("logging test lines", func() {
									Expect(logs).To(ContainSubstring("manifest tests completed successfully"))
								})
							})
						})
					})
				})
			})

			When("stability tests shouldn't be successful", func() {
				const (
					testSuccessLogLine = "stability tests completed successfully"
				)
				var fakeMobyClient *fakes.MobyClient

				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testSuccessLogLine, 0)
				})

				It("exits with an error if env vars are incorrectly formatted", func() {
					subjectUnderTest := commands.NewTileTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					err := subjectUnderTest.Execute([]string{"--stability", "--verbose", "--tile-path", helloTilePath, "--ginkgo-flags", "-r -slowSpecThreshold 1", "--environment-variable", "MISFORMATTED_ENV_VAR"})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Environment variables must have the format [key]=[value]"))
				})
			})

			When("all tests are run", func() {
				var fakeMobyClient *fakes.MobyClient
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient("success", 0)
				})
				When("executing migration tests", func() {
					var subjectUnderTest commands.TileTest
					BeforeEach(func() {
						writer.Reset()
						subjectUnderTest = commands.NewTileTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					})

					It("succeeds", func() {
						err := subjectUnderTest.Execute([]string{"--tile-path", helloTilePath})
						Expect(err).To(BeNil())

						By("creating a test container", func() {
							Expect(fakeMobyClient.ContainerCreateCallCount()).To(Equal(1))
							_, config, _, _, _, _ := fakeMobyClient.ContainerCreateArgsForCall(0)

							By("executing the tests", func() {
								dockerCmd := "cd /tas/hello-tile/migrations && npm install && npm test && cd /tas/hello-tile && ginkgo -r -p -slowSpecThreshold 15 /tas/hello-tile/test/stability /tas/hello-tile/test/manifest"
								actual := config.Env
								expected := []string{
									"TAS_CONFIG_FILE=/tas/hello-tile/test/manifest/fixtures/tas_config.yml",
									"PRODUCT=hello-tile",
									"RENDERER=ops-manifest",
									"TAS_METADATA_PATH=/tas/hello-tile/test/manifest/fixtures/tas_metadata.yml",
								}

								sort.Strings(expected)
								sort.Strings(actual)
								Expect(actual).To(Equal(expected))
								Expect(config.Cmd).To(Equal(strslice.StrSlice{"/bin/bash", "-c", dockerCmd}))
							})
						})
					})
				})
			})

			When("migration tests should be successful", func() {
				const (
					testSuccessLogLine = "migration tests completed successfully"
				)
				var fakeMobyClient *fakes.MobyClient
				BeforeEach(func() {
					fakeMobyClient = setupFakeMobyClient(testSuccessLogLine, 0)
				})
				When("executing migration tests", func() {
					var subjectUnderTest commands.TileTest
					BeforeEach(func() {
						writer.Reset()
						subjectUnderTest = commands.NewTileTest(logger, ctx, fakeMobyClient, fakeSshProvider)
					})

					It("succeeds and logs info", func() {
						err := subjectUnderTest.Execute([]string{"--migrations", "--verbose", "--tile-path", helloTilePath})
						Expect(err).To(BeNil())

						By("logging helpful messages", func() {
							logs := writer.String()
							By("logging container information", func() {
								Expect(logs).To(ContainSubstring("Building / restoring cached docker image"))
							})
							By("logging test lines", func() {
								Expect(logs).To(ContainSubstring("migration tests completed successfully"))
							})
						})

						By("creating a test container", func() {
							Expect(fakeMobyClient.ContainerCreateCallCount()).To(Equal(1))
							_, config, _, _, _, _ := fakeMobyClient.ContainerCreateArgsForCall(0)

							By("executing the tests", func() {
								dockerCmd := "cd /tas/hello-tile/migrations && npm install && npm test"
								Expect(config.Cmd).To(Equal(strslice.StrSlice{"/bin/bash", "-c", dockerCmd}))
							})
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
			subjectUnderTest := commands.NewTileTest(logger, ctx, fakeMobyClient, &fakeSshThinger)
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

	rc := io.NopCloser(strings.NewReader(`{"error": "", "message": "tagged kiln_test_dependencies:vmware"}`))
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
	Fatal(args ...any)
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
	if err := os.MkdirAll(fixturesDirectory, 0o766); err != nil {
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
