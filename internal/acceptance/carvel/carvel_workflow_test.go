package acceptance_test

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/pivotal-cf/kiln/pkg/bake"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v3"
)

// mockArtifactory is a test HTTP server that faithfully simulates Artifactory's
// upload (PUT) and download (GET) behaviour, including Basic Auth verification.
// Upload stores the tarball bytes keyed by request path; download serves them
// back.  The /artifactory prefix that the real download client prepends is
// handled transparently.
type mockArtifactory struct {
	mu       sync.Mutex
	blobs    map[string][]byte
	server   *httptest.Server
	username string
	password string

	putCount int
	getCount int
}

func newMockArtifactory(username, password string) *mockArtifactory {
	m := &mockArtifactory{
		blobs:    make(map[string][]byte),
		username: username,
		password: password,
	}
	m.server = httptest.NewServer(m)
	return m
}

func (m *mockArtifactory) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, p, ok := r.BasicAuth()
	if !ok || u != m.username || p != m.password {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Normalize path: strip the /artifactory prefix the download client adds
	key := r.URL.Path
	key = strings.TrimPrefix(key, "/artifactory")

	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m.mu.Lock()
		m.blobs[key] = body
		m.putCount++
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)

	case http.MethodGet:
		m.mu.Lock()
		data, found := m.blobs[key]
		m.getCount++
		m.mu.Unlock()
		if !found {
			http.Error(w, fmt.Sprintf("not found: %s (have: %v)", key, m.storedKeys()), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(data)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *mockArtifactory) storedKeys() []string {
	keys := make([]string, 0, len(m.blobs))
	for k := range m.blobs {
		keys = append(keys, k)
	}
	return keys
}

func (m *mockArtifactory) Close() { m.server.Close() }
func (m *mockArtifactory) URL() string { return m.server.URL }

func (m *mockArtifactory) PutCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.putCount
}

func (m *mockArtifactory) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getCount
}

// ---------------------------------------------------------------------------
// End-to-end acceptance test for the full Carvel tile developer workflow.
//
// This test exercises every `kiln carvel` subcommand in sequence, chaining
// their outputs exactly like a real developer and CI pipeline would:
//
//   Step 1  kiln carvel bake          (local bake, no Kilnfile.lock)
//   Step 2  kiln carvel upload        (creates BOSH release, uploads, writes lock)
//           git add + commit          (Kilnfile.lock)
//   Step 3  kiln carvel bake          (CI-style: downloads from lock)
//   Step 4  kiln carvel publish --final (downloads, bakes, writes bake record)
//           git add + commit          (bake_records/)
//   Step 5  kiln carvel rebake        (re-bakes from record, verifies checksum)
//
// A mock Artifactory server stores the actual uploaded tarball and serves it
// back on download, proving the full round-trip.
// ---------------------------------------------------------------------------
var _ = Describe("carvel full workflow", Ordered, func() {
	const (
		sampleTileFixture = "fixtures/sample-tile"
		artUsername        = "test-user"
		artPassword        = "test-pass"
		artRepo            = "test-repo"
	)

	var (
		tmpDir    string
		inputPath string
		art       *mockArtifactory
	)

	variableFlags := func() []string {
		return []string{
			"--variable", "artifactory_host=" + art.URL(),
			"--variable", "artifactory_repo=" + artRepo,
			"--variable", "artifactory_username=" + artUsername,
			"--variable", "artifactory_password=" + artPassword,
		}
	}

	gitInTile := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = inputPath
		out, err := cmd.CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v failed: %s", args, string(out))
	}

	currentSHA := func() string {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = inputPath
		out, err := cmd.Output()
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return strings.TrimSpace(string(out))
	}

	tileChecksum := func(path string) string {
		f, err := os.Open(path)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		defer func() { _ = f.Close() }()
		h := sha256.New()
		_, err = io.Copy(h, f)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return hex.EncodeToString(h.Sum(nil))
	}

	assertValidTile := func(pivotalPath string) {
		archive, err := os.Open(pivotalPath)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		defer func() { _ = archive.Close() }()

		info, err := archive.Stat()
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		zr, err := zip.NewReader(archive, info.Size())
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		_, err = zr.Open("metadata/metadata.yml")
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "tile must contain metadata/metadata.yml")

		var hasRelease bool
		for _, f := range zr.File {
			if filepath.Dir(f.Name) == "releases" && filepath.Ext(f.Name) == ".tgz" {
				hasRelease = true
				break
			}
		}
		ExpectWithOffset(1, hasRelease).To(BeTrue(), "tile must contain releases/*.tgz")
	}

	BeforeAll(func() {
		if _, err := exec.LookPath("bosh"); err != nil {
			Skip("bosh CLI not installed — skipping carvel workflow acceptance tests")
		}

		var err error
		tmpDir, err = os.MkdirTemp("", "kiln-carvel-workflow-*")
		Expect(err).NotTo(HaveOccurred())

		inputPath = filepath.Join(tmpDir, "tile")
		err = os.CopyFS(inputPath, os.DirFS(sampleTileFixture))
		Expect(err).NotTo(HaveOccurred())

		art = newMockArtifactory(artUsername, artPassword)

		gitInTile("init")
		gitInTile("config", "user.email", "test@test.com")
		gitInTile("config", "user.name", "Test")
		gitInTile("add", ".")
		gitInTile("commit", "-m", "initial commit")
	})

	AfterAll(func() {
		if art != nil {
			art.Close()
		}
		_ = os.RemoveAll(tmpDir)
	})

	// -----------------------------------------------------------------------
	// Step 1: Local bake (no Kilnfile.lock, no Artifactory interaction)
	// -----------------------------------------------------------------------
	It("Step 1: bakes a tile locally without Kilnfile.lock", func() {
		outputFile := filepath.Join(tmpDir, "step1.pivotal")

		cmd := exec.Command(pathToMain,
			append([]string{
				"carvel", "bake",
				"--source-directory", inputPath,
				"--output-file", outputFile,
				"--verbose",
			}, variableFlags()...)...,
		)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, "120s").Should(gexec.Exit(0))

		assertValidTile(outputFile)

		Expect(art.PutCount()).To(Equal(0), "local bake must not upload anything")
		Expect(art.GetCount()).To(Equal(0), "local bake must not download anything")

		_, err = os.Stat(filepath.Join(inputPath, "Kilnfile.lock"))
		Expect(os.IsNotExist(err)).To(BeTrue(), "local bake must not create Kilnfile.lock")
	})

	// -----------------------------------------------------------------------
	// Step 2: Upload (creates BOSH release, uploads to Artifactory, writes lock)
	// -----------------------------------------------------------------------
	It("Step 2: uploads the BOSH release to Artifactory and writes Kilnfile.lock", func() {
		cmd := exec.Command(pathToMain,
			append([]string{
				"carvel", "upload",
				"--source-directory", inputPath,
				"--verbose",
			}, variableFlags()...)...,
		)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, "120s").Should(gexec.Exit(0))

		Expect(art.PutCount()).To(Equal(1), "upload must PUT exactly once")

		lockfilePath := filepath.Join(inputPath, "Kilnfile.lock")
		Expect(lockfilePath).To(BeAnExistingFile())

		lockData, err := os.ReadFile(lockfilePath)
		Expect(err).NotTo(HaveOccurred())

		var lock cargo.KilnfileLock
		Expect(yaml.Unmarshal(lockData, &lock)).To(Succeed())

		Expect(lock.Releases).To(HaveLen(1))
		rel := lock.Releases[0]
		Expect(rel.Name).To(Equal("k8s-tile-test"))
		Expect(rel.Version).To(Equal("0.1.1"))
		Expect(rel.SHA1).NotTo(BeEmpty(), "lock must contain SHA1 of uploaded tarball")
		Expect(rel.RemoteSource).To(Equal("artifactory"))
		Expect(rel.RemotePath).To(Equal("bosh-releases/k8s-tile-test/k8s-tile-test-0.1.1.tgz"))

		gitInTile("add", "Kilnfile.lock")
		gitInTile("commit", "-m", "add Kilnfile.lock from upload")
	})

	// -----------------------------------------------------------------------
	// Step 3: CI-style bake (downloads cached BOSH release via Kilnfile.lock)
	// -----------------------------------------------------------------------
	It("Step 3: bakes a tile using Kilnfile.lock (CI path with Artifactory download)", func() {
		outputFile := filepath.Join(tmpDir, "step3-ci.pivotal")

		getCountBefore := art.GetCount()

		cmd := exec.Command(pathToMain,
			append([]string{
				"carvel", "bake",
				"--source-directory", inputPath,
				"--output-file", outputFile,
				"--verbose",
			}, variableFlags()...)...,
		)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, "120s").Should(gexec.Exit(0))

		assertValidTile(outputFile)

		Expect(art.GetCount()).To(BeNumerically(">", getCountBefore),
			"CI bake must download the cached BOSH release from Artifactory")
	})

	// -----------------------------------------------------------------------
	// Step 4: Publish --final (downloads, bakes, creates bake record)
	// -----------------------------------------------------------------------
	var publishChecksum string

	It("Step 4: publishes a final tile and writes a bake record", func() {
		outputFile := filepath.Join(tmpDir, "step4-final.pivotal")

		cmd := exec.Command(pathToMain,
			append([]string{
				"carvel", "publish",
				"--source-directory", inputPath,
				"--output-file", outputFile,
				"--final",
				"--verbose",
			}, variableFlags()...)...,
		)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, "120s").Should(gexec.Exit(0))

		assertValidTile(outputFile)
		publishChecksum = tileChecksum(outputFile)

		recordsDir := filepath.Join(inputPath, "bake_records")
		Expect(recordsDir).To(BeADirectory())

		recordPath := filepath.Join(recordsDir, "0.1.1.json")
		Expect(recordPath).To(BeAnExistingFile())

		recordData, err := os.ReadFile(recordPath)
		Expect(err).NotTo(HaveOccurred())

		var record bake.Record
		Expect(json.Unmarshal(recordData, &record)).To(Succeed())

		Expect(record.Version).To(Equal("0.1.1"))
		Expect(record.SourceRevision).To(Equal(currentSHA()),
			"bake record source_revision must match current HEAD")
		Expect(record.FileChecksum).NotTo(BeEmpty())
		Expect(record.FileChecksum).To(Equal(publishChecksum),
			"bake record checksum must match the actual tile file")

		// Do NOT commit bake_records yet — rebake must run at the same
		// HEAD that publish captured.  In a real CI pipeline the
		// Concourse resource checks out the commit from the record.
	})

	// -----------------------------------------------------------------------
	// Step 5: Rebake from bake record (reproducibility verification)
	// -----------------------------------------------------------------------
	It("Step 5: re-bakes from the bake record with an identical checksum", func() {
		outputFile := filepath.Join(tmpDir, "step5-rebake.pivotal")
		recordPath := filepath.Join(inputPath, "bake_records", "0.1.1.json")

		args := append([]string{
			"carvel", "rebake",
			"--output-file", outputFile,
			"--verbose",
		}, variableFlags()...)
		args = append(args, recordPath)
		cmd := exec.Command(pathToMain, args...)
		cmd.Dir = inputPath
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, "120s").Should(gexec.Exit(0))

		assertValidTile(outputFile)

		rebakeChecksum := tileChecksum(outputFile)
		Expect(rebakeChecksum).To(Equal(publishChecksum),
			"rebake must produce a byte-for-byte identical tile to publish")
	})

	// -----------------------------------------------------------------------
	// Meta-assertions: verify the mock was exercised correctly across the
	// entire workflow.
	// -----------------------------------------------------------------------
	It("exercised Artifactory correctly across the full workflow", func() {
		Expect(art.PutCount()).To(Equal(1),
			"exactly one upload should have occurred across the entire workflow")
		Expect(art.GetCount()).To(BeNumerically(">=", 2),
			"at least two downloads should have occurred (CI bake + publish)")
	})
})
