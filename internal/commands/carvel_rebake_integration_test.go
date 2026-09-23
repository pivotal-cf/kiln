//go:build integration

package commands_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/pkg/bake"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v3"
)

// Integration specs for CarvelReBake; drive the real bosh CLI end to end.
var _ = Describe("CarvelReBake (integration)", func() {
	var (
		outLogger *log.Logger
		errLogger *log.Logger
		command   commands.CarvelReBake
	)

	BeforeEach(func() {
		outLogger = log.New(GinkgoWriter, "", 0)
		errLogger = log.New(GinkgoWriter, "", 0)
		command = commands.NewCarvelReBake(outLogger, errLogger)
	})

	Describe("Execute", func() {
		When("a valid bake record and mock Artifactory are provided", func() {
			var (
				inputPath  string
				outputPath string
				recordPath string
				server     *httptest.Server
			)

			BeforeEach(func() {

				var err error
				inputPath, err = os.MkdirTemp("", "rebake-happy-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				err = os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))
				Expect(err).NotTo(HaveOccurred())

				gitCmd := func(args ...string) {
					// Pin the identity so commits work on a CI runner with no
					// global git user configured.
					ident := []string{"-c", "user.name=test", "-c", "user.email=test@test.com"}
					cmd := exec.Command("git", append(ident, args...)...)
					cmd.Dir = inputPath
					out, err := cmd.CombinedOutput()
					ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, out)
				}

				gitCmd("init")
				gitCmd("add", ".")
				gitCmd("commit", "-m", "initial commit")

				b := carvel.NewBaker()
				b.SetWriter(GinkgoWriter)
				Expect(b.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, carvel.BakeOptions{})).To(Succeed())
				tarball, err := b.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())
				tarballData, err := os.ReadFile(tarball)
				Expect(err).NotTo(HaveOccurred())
				releaseVersion := b.GetReleaseVersion()

				var (
					mu    sync.Mutex
					blobs = make(map[string][]byte)
				)
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					key := strings.TrimPrefix(r.URL.Path, "/artifactory")
					switch r.Method {
					case http.MethodPut:
						body, _ := io.ReadAll(r.Body)
						mu.Lock()
						blobs[key] = body
						mu.Unlock()
						w.WriteHeader(http.StatusCreated)
					case http.MethodGet:
						mu.Lock()
						data, ok := blobs[key]
						mu.Unlock()
						if !ok {
							http.Error(w, "not found", http.StatusNotFound)
							return
						}
						w.Header().Set("Content-Type", "application/gzip")
						_, _ = w.Write(data)
					}
				}))

				// Pre-load the mock with the tarball at the expected path
				remotePath := "/test-repo/bosh-releases/k8s-tile-test-pkg/k8s-tile-test-pkg-" + releaseVersion + ".tgz"
				blobs[remotePath] = tarballData

				kf := cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{{
						Type:            "artifactory",
						ArtifactoryHost: server.URL,
						Repo:            "test-repo",
						Username:        "user",
						Password:        "pass",
						PathTemplate:    "bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz",
					}},
				}
				kfData, err := yaml.Marshal(&kf)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile"), kfData, 0644)).To(Succeed())

				lock := cargo.KilnfileLock{
					Releases: []cargo.BOSHReleaseTarballLock{{
						Name:         "k8s-tile-test-pkg",
						Version:      releaseVersion,
						RemotePath:   "bosh-releases/k8s-tile-test-pkg/k8s-tile-test-pkg-" + releaseVersion + ".tgz",
						RemoteSource: "artifactory",
					}},
					Stemcell: cargo.Stemcell{OS: "ubuntu-jammy", Version: "1.446"},
				}
				lockData, err := yaml.Marshal(&lock)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), lockData, 0644)).To(Succeed())

				gitCmd("add", ".")
				gitCmd("commit", "-m", "add kilnfiles")

				sha := strings.TrimSpace(func() string {
					cmd := exec.Command("git", "rev-parse", "HEAD")
					cmd.Dir = inputPath
					out, _ := cmd.Output()
					return string(out)
				}())

				record := bake.Record{
					SourceRevision: sha,
					Version:        "0.1.1",
					TileDirectory:  inputPath,
				}
				buf, err := json.Marshal(record)
				Expect(err).NotTo(HaveOccurred())

				recordPath = filepath.Join(filepath.Dir(inputPath), "record.json")
				Expect(os.WriteFile(recordPath, buf, 0644)).To(Succeed())

				outputPath = filepath.Join(filepath.Dir(inputPath), "output.pivotal")
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
				if server != nil {
					server.Close()
				}
			})

			It("re-bakes successfully from the bake record", func() {
				err := command.Execute([]string{
					"--output-file", outputPath,
					"--verbose",
					recordPath,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())
			})
		})

		// Regression test: runs with CWD != tile source dir, so it only
		// passes when resolveReleasesDirectory anchors ReleasesDirectory
		// to sourcePath rather than the process's CWD.
		When("the process CWD differs from the tile's source directory and an additional-release cache exists there", func() {
			const (
				extraReleaseName    = "smoke-test-scripts"
				extraReleaseVersion = "dev"
				extraJobName        = "smoke-test-scripts"
			)

			var (
				inputPath      string
				outputPath     string
				recordPath     string
				server         *httptest.Server
				extraFetchHits int
			)

			BeforeEach(func() {
				extraFetchHits = 0

				var err error
				inputPath, err = os.MkdirTemp("", "rebake-cache-*")
				Expect(err).NotTo(HaveOccurred())
				inputPath += "/tile"
				Expect(os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))).To(Succeed())

				gitCmd := func(args ...string) {
					ident := []string{"-c", "user.name=test", "-c", "user.email=test@test.com"}
					cmd := exec.Command("git", append(ident, args...)...)
					cmd.Dir = inputPath
					out, err := cmd.CombinedOutput()
					ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, out)
				}

				gitCmd("init")
				gitCmd("add", ".")
				gitCmd("commit", "-m", "initial commit")

				b := carvel.NewBaker()
				b.SetWriter(GinkgoWriter)
				Expect(b.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, carvel.BakeOptions{})).To(Succeed())
				tarball, err := b.GetReleaseTarball()
				Expect(err).NotTo(HaveOccurred())
				tarballData, err := os.ReadFile(tarball)
				Expect(err).NotTo(HaveOccurred())
				releaseVersion := b.GetReleaseVersion()

				baseYMLPath := filepath.Join(inputPath, "base.yml")
				raw, err := os.ReadFile(baseYMLPath)
				Expect(err).NotTo(HaveOccurred())
				var m models.Metadata
				Expect(yaml.Unmarshal(raw, &m)).To(Succeed())
				m.AdditionalReleases = []models.AdditionalRelease{
					{Name: extraReleaseName, Jobs: []models.AdditionalJob{{Name: extraJobName}}},
				}
				updated, err := yaml.Marshal(&m)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(baseYMLPath, updated, 0644)).To(Succeed())

				extraRemotePath := "/artifactory/test-repo/bosh-releases/" + extraReleaseName + "/" + extraReleaseName + "-" + extraReleaseVersion + ".tgz"
				ownRemotePath := "/artifactory/test-repo/bosh-releases/k8s-tile-test-pkg/k8s-tile-test-pkg-" + releaseVersion + ".tgz"

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case ownRemotePath:
						w.Header().Set("Content-Type", "application/gzip")
						_, _ = w.Write(tarballData)
					case extraRemotePath:
						// Must be served from the local cache; fail loudly
						// if kiln requests it over the network.
						extraFetchHits++
						http.Error(w, "additional release should have been served from local cache, not fetched", http.StatusInternalServerError)
					default:
						http.Error(w, "not found", http.StatusNotFound)
					}
				}))

				kf := cargo.Kilnfile{
					ReleaseSources: []cargo.ReleaseSourceConfig{{
						Type:            "artifactory",
						ArtifactoryHost: server.URL,
						Repo:            "test-repo",
						PathTemplate:    "bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz",
					}},
				}
				kfData, err := yaml.Marshal(&kf)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile"), kfData, 0644)).To(Succeed())

				lock := cargo.KilnfileLock{
					Releases: []cargo.BOSHReleaseTarballLock{
						{
							Name:         "k8s-tile-test-pkg",
							Version:      releaseVersion,
							RemotePath:   "bosh-releases/k8s-tile-test-pkg/k8s-tile-test-pkg-" + releaseVersion + ".tgz",
							RemoteSource: "artifactory",
						},
						{
							Name:         extraReleaseName,
							Version:      extraReleaseVersion,
							RemotePath:   "bosh-releases/" + extraReleaseName + "/" + extraReleaseName + "-" + extraReleaseVersion + ".tgz",
							RemoteSource: "artifactory",
						},
					},
					Stemcell: cargo.Stemcell{OS: "ubuntu-jammy", Version: "1.446"},
				}
				lockData, err := yaml.Marshal(&lock)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(inputPath, "Kilnfile.lock"), lockData, 0644)).To(Succeed())

				// Pre-fetched cache, anchored to the tile's own source
				// directory — never to the process's CWD.
				releasesDir := filepath.Join(inputPath, "releases")
				Expect(os.MkdirAll(releasesDir, 0755)).To(Succeed())
				cachedTarball := filepath.Join(releasesDir, extraReleaseName+"-"+extraReleaseVersion+".tgz")
				// Legacy `kiln bake` parses this as a real BOSH release
				// tarball (cargo.OpenBOSHReleaseTarball).
				Expect(os.WriteFile(cachedTarball, buildMinimalReleaseTarball(extraReleaseName, extraReleaseVersion), 0644)).To(Succeed())

				gitCmd("add", ".")
				gitCmd("commit", "-m", "add kilnfiles and additional_releases")

				sha := strings.TrimSpace(func() string {
					cmd := exec.Command("git", "rev-parse", "HEAD")
					cmd.Dir = inputPath
					out, _ := cmd.Output()
					return string(out)
				}())

				record := bake.Record{
					SourceRevision: sha,
					Version:        "0.1.1",
					TileDirectory:  inputPath,
				}
				buf, err := json.Marshal(record)
				Expect(err).NotTo(HaveOccurred())

				recordPath = filepath.Join(filepath.Dir(inputPath), "record.json")
				Expect(os.WriteFile(recordPath, buf, 0644)).To(Succeed())

				outputPath = filepath.Join(filepath.Dir(inputPath), "output.pivotal")
			})

			AfterEach(func() {
				if inputPath != "" {
					_ = os.RemoveAll(filepath.Dir(inputPath))
				}
				if server != nil {
					server.Close()
				}
			})

			It("serves the additional release from the tile's own releases/ cache instead of fetching it", func() {
				err := command.Execute([]string{
					"--output-file", outputPath,
					"--verbose",
					recordPath,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())
				Expect(extraFetchHits).To(Equal(0), "the additional release was fetched over the network instead of being served from the sourcePath-anchored cache")
			})
		})

		var (
			inputPath          string
			outputPath         string
			recordPath         string
			publishedChecksum  string
			processCWD         string
			originalWorkingDir string
		)

		buildRebakeChecksumFixture := func(recordChecksum func(publishedChecksum string) string) {
			var err error
			inputPath, err = os.MkdirTemp("", "rebake-checksum-*")
			Expect(err).NotTo(HaveOccurred())
			inputPath += "/tile"
			Expect(os.CopyFS(inputPath, os.DirFS("../carvel/testdata/sample-tile"))).To(Succeed())

			gitCmd := func(args ...string) {
				ident := []string{"-c", "user.name=test", "-c", "user.email=test@test.com"}
				cmd := exec.Command("git", append(ident, args...)...)
				cmd.Dir = inputPath
				out, err := cmd.CombinedOutput()
				ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, out)
			}
			gitCmd("init")
			gitCmd("add", ".")
			gitCmd("commit", "-m", "initial commit")

			sha := strings.TrimSpace(func() string {
				cmd := exec.Command("git", "rev-parse", "HEAD")
				cmd.Dir = inputPath
				out, _ := cmd.Output()
				return string(out)
			}())

			// Simulate `kiln carvel publish --final`: bake once and
			// checksum the resulting tile.
			b := carvel.NewBaker()
			b.SetWriter(GinkgoWriter)
			Expect(b.Bake(inputPath, cargo.Kilnfile{}, cargo.KilnfileLock{}, carvel.BakeOptions{})).To(Succeed())
			publishedPath := filepath.Join(filepath.Dir(inputPath), "published.pivotal")
			Expect(b.KilnBake(publishedPath)).To(Succeed())
			var checksumErr error
			publishedChecksum, checksumErr = sha256HexFile(publishedPath)
			Expect(checksumErr).NotTo(HaveOccurred())

			record := bake.Record{
				SourceRevision: sha,
				Version:        "0.1.1",
				FileChecksum:   recordChecksum(publishedChecksum),
				TileDirectory:  inputPath,
			}
			buf, err := json.Marshal(record)
			Expect(err).NotTo(HaveOccurred())

			recordPath = filepath.Join(filepath.Dir(inputPath), "record.json")
			Expect(os.WriteFile(recordPath, buf, 0644)).To(Succeed())

			outputPath = filepath.Join(filepath.Dir(inputPath), "rebaked.pivotal")

			// Run the rebake from a CWD that is neither the tile
			// source directory nor its parent, matching TNZ-153678.
			processCWD, err = os.MkdirTemp("", "rebake-checksum-cwd-*")
			Expect(err).NotTo(HaveOccurred())
			originalWorkingDir, err = os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chdir(processCWD)).To(Succeed())
		}

		cleanUpRebakeChecksumFixture := func() {
			if originalWorkingDir != "" {
				_ = os.Chdir(originalWorkingDir)
			}
			if inputPath != "" {
				_ = os.RemoveAll(filepath.Dir(inputPath))
			}
			if processCWD != "" {
				_ = os.RemoveAll(processCWD)
			}
		}

		When("the bake record's file checksum matches the rebaked tile", func() {
			BeforeEach(func() {
				buildRebakeChecksumFixture(func(publishedChecksum string) string { return publishedChecksum })
			})

			AfterEach(cleanUpRebakeChecksumFixture)

			It("succeeds", func() {
				err := command.Execute([]string{
					"--output-file", outputPath,
					"--verbose",
					recordPath,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(outputPath).To(BeAnExistingFile())
			})
		})

		When("the bake record's file checksum does not match the rebaked tile", func() {
			BeforeEach(func() {
				buildRebakeChecksumFixture(func(string) string { return "sha256-of-a-different-build-entirely" })
			})

			AfterEach(cleanUpRebakeChecksumFixture)

			It("fails with a checksum mismatch error instead of silently accepting the tile", func() {
				err := command.Execute([]string{
					"--output-file", outputPath,
					"--verbose",
					recordPath,
				})
				Expect(err).To(MatchError(ContainSubstring("tile checksum mismatch")))
			})
		})
	})
})

// buildMinimalReleaseTarball builds the smallest tarball
// cargo.OpenBOSHReleaseTarball will accept: a gzip+tar archive containing a
// single release.MF entry with just enough YAML for a name/version.
func buildMinimalReleaseTarball(name, version string) []byte {
	manifest := "name: " + name + "\nversion: " + version + "\n"

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	ExpectWithOffset(1, tw.WriteHeader(&tar.Header{
		Name: "release.MF",
		Mode: 0644,
		Size: int64(len(manifest)),
	})).To(Succeed())
	_, err := tw.Write([]byte(manifest))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, tw.Close()).To(Succeed())

	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, err = gw.Write(tarBuf.Bytes())
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, gw.Close()).To(Succeed())

	return gzBuf.Bytes()
}

func sha256HexFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
