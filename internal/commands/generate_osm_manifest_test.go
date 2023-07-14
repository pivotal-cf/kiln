package commands_test

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v50/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func TestOSM_Execute(t *testing.T) {
	t.Run("it outputs an OSM File", func(t *testing.T) {
		RegisterTestingT(t)

		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseTarballSpecification{
				{
					Name:             "banana",
					GitHubRepository: "https://github.com/cloudfoundry/banana",
				},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.BOSHReleaseTarballLock{
				{Name: "banana", Version: "1.2.3"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		rs := new(fakes.ReleaseStorage)
		rs.FindReleaseVersionReturns(cargo.BOSHReleaseTarballLock{}, nil)

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()

		logger := log.New(outBuffer, "", 0)
		cmd := commands.NewOSM(logger, rs)
		err := cmd.Execute([]string{"--no-download", "--kilnfile", kfp})

		Expect(err).ToNot(HaveOccurred())

		Eventually(outBuffer).Should(gbytes.Say("other:banana:1.2.3:"), "output should contain special formatted row")
		Eventually(outBuffer).Should(gbytes.Say("  name: banana"))
		Eventually(outBuffer).Should(gbytes.Say("  version: 1.2.3"))
		Eventually(outBuffer).Should(gbytes.Say("  repository: Other"))
		Eventually(outBuffer).Should(gbytes.Say("  url: https://github.com/cloudfoundry/banana"))
		Eventually(outBuffer).Should(gbytes.Say("  license: Apache2.0"))
		Eventually(outBuffer).Should(gbytes.Say("  interactions:"))
		Eventually(outBuffer).Should(gbytes.Say("  - Distributed - Calling Existing Classes"))
		Eventually(outBuffer).Should(gbytes.Say("  other-distribution: ./banana-1.2.3.zip"))
	})

	t.Run("it excludes non-cloudfoundry entries", func(t *testing.T) {
		RegisterTestingT(t)

		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseTarballSpecification{
				{
					Name:             "banana",
					GitHubRepository: "https://github.com/cloudfoundry/banana",
				},
				{
					Name:             "apple",
					GitHubRepository: "https://github.com/pivotal/apple",
				},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.BOSHReleaseTarballLock{
				{Name: "banana", Version: "1.2.3"},
				{Name: "apple", Version: "1.2.4"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		rs := new(fakes.ReleaseStorage)
		rs.FindReleaseVersionCalls(func(spec cargo.BOSHReleaseTarballSpecification, _ bool) (cargo.BOSHReleaseTarballLock, error) {
			switch spec.Name {
			case "banana":
				return cargo.BOSHReleaseTarballLock{}, nil
			}
			return cargo.BOSHReleaseTarballLock{}, component.ErrNotFound
		})

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()

		logger := log.New(outBuffer, "", 0)
		cmd := commands.NewOSM(logger, rs)

		Expect(cmd.Execute([]string{"--no-download", "--kilnfile", kfp})).Error().ToNot(HaveOccurred())

		Consistently(outBuffer).ShouldNot(gbytes.Say("other:apple:1.2.4:"), "output should omit entry for non-cf repos")
		Eventually(outBuffer).Should(gbytes.Say("other:banana:1.2.3:"), "output should contain cf entry")
	})

	t.Run("it finds buildpacks if they contain \"-offline\"", func(t *testing.T) {
		RegisterTestingT(t)

		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseTarballSpecification{
				{
					Name: "lemon-offline-buildpack",
				},
				{
					Name:             "banana",
					GitHubRepository: "https://github.com/pivotal/banana",
				},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.BOSHReleaseTarballLock{
				{Name: "lemon-offline-buildpack", Version: "1.2.3"},
				{Name: "banana", Version: "1.2.3"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		rs := new(fakes.ReleaseStorage)
		rs.FindReleaseVersionCalls(func(spec cargo.BOSHReleaseTarballSpecification, _ bool) (cargo.BOSHReleaseTarballLock, error) {
			switch spec.Name {
			case "lemon-buildpack":
				return cargo.BOSHReleaseTarballLock{
					RemotePath: "https://bosh.io/d/github.com/cloudfoundry/lemon-buildpack-release?v=1.2.3",
				}, nil
			case "banana":
				return cargo.BOSHReleaseTarballLock{}, nil
			}
			return cargo.BOSHReleaseTarballLock{}, component.ErrNotFound
		})

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()

		logger := log.New(outBuffer, "", 0)
		cmd := commands.NewOSM(logger, rs)

		Expect(cmd.Execute([]string{"--no-download", "--kilnfile", kfp})).Error().ToNot(HaveOccurred())

		Eventually(outBuffer).Should(gbytes.Say("other:lemon-offline-buildpack:1.2.3:"), "output should contain special formatted row")
		Eventually(outBuffer).Should(gbytes.Say("  name: lemon-offline-buildpack"))
		Eventually(outBuffer).Should(gbytes.Say("  version: 1.2.3"))
		Eventually(outBuffer).Should(gbytes.Say("  repository: Other"))
		Eventually(outBuffer).Should(gbytes.Say("  url: https://github.com/cloudfoundry/lemon-buildpack-release"))
		Eventually(outBuffer).Should(gbytes.Say("  license: Apache2.0"))
		Eventually(outBuffer).Should(gbytes.Say("  interactions:"))
		Eventually(outBuffer).Should(gbytes.Say("  - Distributed - Calling Existing Classes"))
		Eventually(outBuffer).Should(gbytes.Say("  other-distribution: ./lemon-offline-buildpack-1.2.3.zip"))
	})

	t.Run("it will fetch only a single package if only flag and url is specified", func(t *testing.T) {
		RegisterTestingT(t)

		testPackage := "zebes-tallon"
		testUrl := "https://www.github.com/samus/zebes-tallon"

		splitString := strings.SplitN(testUrl, "/", -1)
		testRepo := splitString[len(splitString)-1]
		testOwner := splitString[len(splitString)-2]

		testVersion := "1.2.3"
		mockClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposReleasesLatestByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write(mock.MustMarshal(github.RepositoryRelease{
						Name: &testVersion,
					}),
					)
					if err != nil {
						return
					}
				}),
			),
		)

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()
		rs := new(fakes.ReleaseStorage)
		logger := log.New(outBuffer, "", 0)
		c := github.NewClient(mockClient)
		ctx := context.Background()
		testLatestRelease, _, _ := c.Repositories.GetLatestRelease(ctx, testOwner, testRepo)
		testName := testLatestRelease.GetName()

		cmd := commands.NewOSMWithGHClient(logger, rs, c)
		Expect(cmd.Execute([]string{"--only", testPackage, "--url", testUrl, "--no-download"})).Error().ToNot(HaveOccurred())

		Eventually(outBuffer).Should(gbytes.Say("other:zebes-tallon:"+testName+":"), "output should contain special formatted row")
		Eventually(outBuffer).Should(gbytes.Say("  name: zebes-tallon"))
		Eventually(outBuffer).Should(gbytes.Say("  version: " + testName))
		Eventually(outBuffer).Should(gbytes.Say("  repository: Other"))
		Eventually(outBuffer).Should(gbytes.Say("  url: https://www.github.com/samus/zebes-tallon"))
		Eventually(outBuffer).Should(gbytes.Say("  license: Apache2.0"))
		Eventually(outBuffer).Should(gbytes.Say("  interactions:"))
		Eventually(outBuffer).Should(gbytes.Say("  - Distributed - Calling Existing Classes"))
		Eventually(outBuffer).Should(gbytes.Say("  other-distribution: ./zebes-tallon-" + testName + ".zip"))
	})

	t.Run("it will fail if only flag is specified without a url flag", func(t *testing.T) {
		RegisterTestingT(t)

		testPackage := "zebes-tallon"
		testVersion := "1.2.3"
		mockClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposReleasesLatestByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write(mock.MustMarshal(github.RepositoryRelease{
						Name: &testVersion,
					}),
					)
					if err != nil {
						return
					}
				}),
			),
		)

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()
		rs := new(fakes.ReleaseStorage)
		logger := log.New(outBuffer, "", 0)
		c := github.NewClient(mockClient)

		cmd := commands.NewOSMWithGHClient(logger, rs, c)
		Expect(cmd.Execute([]string{"--only", testPackage, "--no-download"})).Error().To(MatchError("missing --url, must provide a --url for the Github repository of specified package"))
	})

	t.Run("it will fail if url flag is specified without an only flag", func(t *testing.T) {
		RegisterTestingT(t)

		testUrl := "https://www.github.com/samus/zebes-tallon"
		testVersion := "1.2.3"
		mockClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposReleasesLatestByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write(mock.MustMarshal(github.RepositoryRelease{
						Name: &testVersion,
					}),
					)
					if err != nil {
						return
					}
				}),
			),
		)

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()
		rs := new(fakes.ReleaseStorage)
		logger := log.New(outBuffer, "", 0)
		c := github.NewClient(mockClient)

		cmd := commands.NewOSMWithGHClient(logger, rs, c)
		Expect(cmd.Execute([]string{"--url", testUrl, "--no-download"})).Error().To(MatchError("missing --only, must provide a --only for the specified package of the Github repository"))
	})

	t.Run("it will fail if only flag is specified with an invalid url flag", func(t *testing.T) {
		RegisterTestingT(t)

		testPackage := "zebes-tallon"
		testUrl := "actual-garbage"
		testVersion := "1.2.3"
		mockClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposReleasesLatestByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write(mock.MustMarshal(github.RepositoryRelease{
						Name: &testVersion,
					}),
					)
					if err != nil {
						return
					}
				}),
			),
		)

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()
		rs := new(fakes.ReleaseStorage)
		logger := log.New(outBuffer, "", 0)
		c := github.NewClient(mockClient)

		cmd := commands.NewOSMWithGHClient(logger, rs, c)
		Expect(cmd.Execute([]string{"--only", testPackage, "--url", testUrl, "--no-download"})).Error().To(MatchError("invalid --url, must provide a valid Github --url for specified package"))
	})
}

func writeYAML(t *testing.T, path string, data any) {
	t.Helper()
	buf, err := yaml.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAndIgnoreError(f)

	_, err = f.Write(buf)
	if err != nil {
		t.Fatal(err)
	}
}
