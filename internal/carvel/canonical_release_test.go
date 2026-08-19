package carvel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// testTarFile describes one entry to bake into a synthetic tar/gzip fixture
// built by buildTarGz. Letting each test control ModTime/Uid/Gid/order lets
// us simulate "dirty" bosh-cli output without shelling out to bosh.
type testTarFile struct {
	name       string
	data       string
	typeflag   byte
	mode       int64
	modTime    time.Time
	uid        int
	gid        int
	uname      string
	paxRecords map[string]string
	xattrs     map[string]string
}

func buildTarGz(t *testing.T, files []testTarFile, gzModTime time.Time) []byte {
	t.Helper()

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, f := range files {
		typeflag := f.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		mode := f.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{
			Name:       f.name,
			Typeflag:   typeflag,
			Mode:       mode,
			Size:       int64(len(f.data)),
			ModTime:    f.modTime,
			Uid:        f.uid,
			Gid:        f.gid,
			Uname:      f.uname,
			PAXRecords: f.paxRecords,
			Xattrs:     f.xattrs, //nolint:staticcheck // deliberately testing the deprecated field is stripped too
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", f.name, err)
		}
		if typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(f.data)); err != nil {
				t.Fatalf("write data %s: %v", f.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	var gzBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&gzBuf, gzip.DefaultCompression)
	if err != nil {
		t.Fatalf("new gzip writer: %v", err)
	}
	gw.ModTime = gzModTime
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return gzBuf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// --- readTarGz / writeTarGz ---

func TestReadTarGzRoundTrip(t *testing.T) {
	src := buildTarGz(t, []testTarFile{
		{name: "dir/", typeflag: tar.TypeDir, mode: 0o755, modTime: time.Now(), uid: 501, gid: 20, uname: "someone"},
		{name: "dir/a.txt", data: "hello", modTime: time.Now().Add(-time.Hour), uid: 501, gid: 20, uname: "someone"},
		{name: "b.txt", data: "world"},
	}, time.Now())

	entries, err := readTarGz(src)
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if string(entries["dir/a.txt"].data) != "hello" {
		t.Errorf("dir/a.txt content = %q, want %q", entries["dir/a.txt"].data, "hello")
	}
	if string(entries["b.txt"].data) != "world" {
		t.Errorf("b.txt content = %q, want %q", entries["b.txt"].data, "world")
	}
	if entries["dir/"].header.Typeflag != tar.TypeDir {
		t.Errorf("dir/ typeflag = %v, want TypeDir", entries["dir/"].header.Typeflag)
	}
}

func TestReadTarGzInvalidInput(t *testing.T) {
	if _, err := readTarGz([]byte("not a gzip stream")); err == nil {
		t.Fatal("expected error for non-gzip input, got nil")
	}
}

func TestWriteTarGzCanonicalizesHeaders(t *testing.T) {
	src := buildTarGz(t, []testTarFile{
		{name: "z.txt", data: "z", modTime: time.Now(), uid: 501, gid: 20, uname: "someone"},
		{name: "a.txt", data: "a", modTime: time.Now().Add(-24 * time.Hour), uid: 999, gid: 999, uname: "other"},
	}, time.Now())

	entries, err := readTarGz(src)
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}

	out, err := writeTarGz(entries)
	if err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	gotEntries, err := readTarGz(out)
	if err != nil {
		t.Fatalf("re-read canonicalized output: %v", err)
	}

	for name, e := range gotEntries {
		if !e.header.ModTime.Equal(canonicalEpoch) {
			t.Errorf("%s: ModTime = %v, want %v", name, e.header.ModTime, canonicalEpoch)
		}
		if e.header.Uid != 0 || e.header.Gid != 0 {
			t.Errorf("%s: Uid/Gid = %d/%d, want 0/0", name, e.header.Uid, e.header.Gid)
		}
		if e.header.Uname != "" || e.header.Gname != "" {
			t.Errorf("%s: Uname/Gname = %q/%q, want empty", name, e.header.Uname, e.header.Gname)
		}
	}
}

func TestWriteTarGzSortsEntries(t *testing.T) {
	entries := map[string]tarEntry{
		"z.txt": {header: &tar.Header{Name: "z.txt", Typeflag: tar.TypeReg, Mode: 0o644}, data: []byte("z")},
		"a.txt": {header: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644}, data: []byte("a")},
		"m.txt": {header: &tar.Header{Name: "m.txt", Typeflag: tar.TypeReg, Mode: 0o644}, data: []byte("m")},
	}

	out, err := writeTarGz(entries)
	if err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		names = append(names, hdr.Name)
	}

	want := []string{"a.txt", "m.txt", "z.txt"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("entry order = %v, want %v", names, want)
	}
}

func TestWriteTarGzDeterministicRegardlessOfInputTimestampsOrOrder(t *testing.T) {
	filesA := []testTarFile{
		{name: "a.txt", data: "content-a", modTime: time.Now(), uid: 501, gid: 20, uname: "userA"},
		{name: "b.txt", data: "content-b", modTime: time.Now().Add(-time.Hour), uid: 502, gid: 21, uname: "userB"},
	}
	filesB := []testTarFile{
		// same logical content, different order, different timestamps/ownership
		{name: "b.txt", data: "content-b", modTime: time.Now().Add(3 * time.Second), uid: 0, gid: 0, uname: ""},
		{name: "a.txt", data: "content-a", modTime: time.Now().Add(-30 * time.Minute), uid: 999, gid: 999, uname: "someoneElse"},
	}

	srcA := buildTarGz(t, filesA, time.Now())
	srcB := buildTarGz(t, filesB, time.Now().Add(5*time.Second))

	entriesA, err := readTarGz(srcA)
	if err != nil {
		t.Fatalf("readTarGz A: %v", err)
	}
	entriesB, err := readTarGz(srcB)
	if err != nil {
		t.Fatalf("readTarGz B: %v", err)
	}

	outA, err := writeTarGz(entriesA)
	if err != nil {
		t.Fatalf("writeTarGz A: %v", err)
	}
	outB, err := writeTarGz(entriesB)
	if err != nil {
		t.Fatalf("writeTarGz B: %v", err)
	}

	if !bytes.Equal(outA, outB) {
		t.Errorf("expected byte-identical canonicalized output for logically identical inputs with different timestamps/order/ownership")
	}
}

func TestWriteTarGzStripsPAXRecordsAndXattrs(t *testing.T) {
	src := buildTarGz(t, []testTarFile{
		{
			name: "a.txt",
			data: "hello",
			paxRecords: map[string]string{
				"SCHILY.xattr.security.selinux": "unconfined_u:object_r:user_home_t:s0",
				"comment":                       "machine-specific metadata",
			},
			xattrs: map[string]string{
				"user.test": "value",
			},
		},
	}, time.Now())

	entries, err := readTarGz(src)
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}
	// Sanity check the fixture actually carries the attributes we're testing get stripped.
	if len(entries["a.txt"].header.PAXRecords) == 0 {
		t.Fatal("test fixture setup failed: expected PAXRecords to be present before canonicalization")
	}

	out, err := writeTarGz(entries)
	if err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	gotEntries, err := readTarGz(out)
	if err != nil {
		t.Fatalf("re-read canonicalized output: %v", err)
	}

	e := gotEntries["a.txt"]
	if len(e.header.PAXRecords) != 0 {
		t.Errorf("expected PAXRecords stripped, got %v", e.header.PAXRecords)
	}
	if len(e.header.Xattrs) != 0 { //nolint:staticcheck // asserting the deprecated field is also cleared
		t.Errorf("expected Xattrs stripped, got %v", e.header.Xattrs)
	}
}

func TestWriteTarGzDeterministicDespiteDifferingPAXRecords(t *testing.T) {
	// Two machines producing the same logical content but different
	// machine-specific extended attributes (e.g. SELinux labels) must
	// still canonicalize to byte-identical output.
	filesA := []testTarFile{
		{name: "a.txt", data: "content", paxRecords: map[string]string{"SCHILY.xattr.security.selinux": "label-a"}},
	}
	filesB := []testTarFile{
		{name: "a.txt", data: "content", paxRecords: map[string]string{"SCHILY.xattr.security.selinux": "label-b", "extra": "record"}},
	}

	entriesA, err := readTarGz(buildTarGz(t, filesA, time.Now()))
	if err != nil {
		t.Fatalf("readTarGz A: %v", err)
	}
	entriesB, err := readTarGz(buildTarGz(t, filesB, time.Now()))
	if err != nil {
		t.Fatalf("readTarGz B: %v", err)
	}

	outA, err := writeTarGz(entriesA)
	if err != nil {
		t.Fatalf("writeTarGz A: %v", err)
	}
	outB, err := writeTarGz(entriesB)
	if err != nil {
		t.Fatalf("writeTarGz B: %v", err)
	}

	if !bytes.Equal(outA, outB) {
		t.Error("expected byte-identical canonicalized output despite differing PAX records")
	}
}

// --- canonicalizeNestedTarGz ---

func TestCanonicalizeNestedTarGzDetectsContentChange(t *testing.T) {
	blobA := buildTarGz(t, []testTarFile{{name: "job.MF", data: "name: foo\n", modTime: time.Now()}}, time.Now())
	blobB := buildTarGz(t, []testTarFile{{name: "job.MF", data: "name: bar\n", modTime: time.Now()}}, time.Now())

	canonA, err := canonicalizeNestedTarGz(blobA)
	if err != nil {
		t.Fatalf("canonicalizeNestedTarGz A: %v", err)
	}
	canonB, err := canonicalizeNestedTarGz(blobB)
	if err != nil {
		t.Fatalf("canonicalizeNestedTarGz B: %v", err)
	}

	if bytes.Equal(canonA, canonB) {
		t.Error("expected different canonicalized bytes for different content, got identical output")
	}
}

func TestCanonicalizeNestedTarGzInvalidInput(t *testing.T) {
	if _, err := canonicalizeNestedTarGz([]byte("garbage")); err == nil {
		t.Fatal("expected error for invalid nested blob, got nil")
	}
}

// --- patchReleaseManifestChecksums ---

const testManifest = `name: k8s-tile-test-pkg
version: 0.1.1+abc123
commit_hash: deadbeef
uncommitted_changes: false
jobs:
  - name: registry-data
    version: fingerprint-job
    fingerprint: fingerprint-job
    sha1: sha256:stale-job-sha
    packages:
      - registry-data
packages:
  - name: registry-data
    version: fingerprint-pkg
    fingerprint: fingerprint-pkg
    sha1: sha256:stale-pkg-sha
    dependencies: []
no_compression: false
`

type manifestEntry struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Fingerprint string   `yaml:"fingerprint"`
	SHA1        string   `yaml:"sha1"`
	Packages    []string `yaml:"packages,omitempty"`
}

type manifestDoc struct {
	Name               string          `yaml:"name"`
	Version            string          `yaml:"version"`
	CommitHash         string          `yaml:"commit_hash"`
	UncommittedChanges bool            `yaml:"uncommitted_changes"`
	Jobs               []manifestEntry `yaml:"jobs"`
	Packages           []manifestEntry `yaml:"packages"`
}

func TestPatchReleaseManifestChecksumsHappyPath(t *testing.T) {
	newSHA := map[string]string{
		"jobs/registry-data":     "newjobsha",
		"packages/registry-data": "newpkgsha",
	}

	out, err := patchReleaseManifestChecksums([]byte(testManifest), newSHA)
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}

	var doc manifestDoc
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal patched manifest: %v", err)
	}

	if len(doc.Jobs) != 1 || doc.Jobs[0].SHA1 != "sha256:newjobsha" {
		t.Errorf("job sha1 = %+v, want sha256:newjobsha", doc.Jobs)
	}
	if len(doc.Packages) != 1 || doc.Packages[0].SHA1 != "sha256:newpkgsha" {
		t.Errorf("package sha1 = %+v, want sha256:newpkgsha", doc.Packages)
	}

	// Untouched fields must survive the round-trip unchanged.
	if doc.Name != "k8s-tile-test-pkg" || doc.Version != "0.1.1+abc123" || doc.CommitHash != "deadbeef" {
		t.Errorf("unexpected mutation of untouched top-level fields: %+v", doc)
	}
	if doc.Jobs[0].Fingerprint != "fingerprint-job" || doc.Jobs[0].Version != "fingerprint-job" {
		t.Errorf("job fingerprint/version should be untouched: %+v", doc.Jobs[0])
	}
	if len(doc.Jobs[0].Packages) != 1 || doc.Jobs[0].Packages[0] != "registry-data" {
		t.Errorf("job packages list should be untouched: %+v", doc.Jobs[0].Packages)
	}
}

func TestPatchReleaseManifestChecksumsSameNameDifferentSection(t *testing.T) {
	// Regression test: a job and a package sharing the same name (as in the
	// real sample-tile fixture, both called "registry-data") must get
	// independently patched checksums, keyed by section, not just by name.
	newSHA := map[string]string{
		"jobs/registry-data":     "jobhash",
		"packages/registry-data": "pkghash",
	}

	out, err := patchReleaseManifestChecksums([]byte(testManifest), newSHA)
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}

	var doc manifestDoc
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if doc.Jobs[0].SHA1 == doc.Packages[0].SHA1 {
		t.Fatalf("expected job and package sha1 to differ, both were %q", doc.Jobs[0].SHA1)
	}
	if doc.Jobs[0].SHA1 != "sha256:jobhash" {
		t.Errorf("job sha1 = %q, want sha256:jobhash", doc.Jobs[0].SHA1)
	}
	if doc.Packages[0].SHA1 != "sha256:pkghash" {
		t.Errorf("package sha1 = %q, want sha256:pkghash", doc.Packages[0].SHA1)
	}
}

func TestPatchReleaseManifestChecksumsNoMatches(t *testing.T) {
	out, err := patchReleaseManifestChecksums([]byte(testManifest), map[string]string{})
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}

	var doc manifestDoc
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Jobs[0].SHA1 != "sha256:stale-job-sha" || doc.Packages[0].SHA1 != "sha256:stale-pkg-sha" {
		t.Errorf("expected sha1 fields unchanged when no match found, got jobs=%q packages=%q",
			doc.Jobs[0].SHA1, doc.Packages[0].SHA1)
	}
}

func TestPatchReleaseManifestChecksumsMissingJobsAndPackagesKeys(t *testing.T) {
	minimal := "name: empty-release\nversion: 1.0.0\n"
	out, err := patchReleaseManifestChecksums([]byte(minimal), map[string]string{"jobs/foo": "x"})
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums should not error when jobs/packages keys are absent: %v", err)
	}
	if !strings.Contains(string(out), "empty-release") {
		t.Errorf("expected manifest content preserved, got: %s", out)
	}
}

func TestPatchReleaseManifestChecksumsEntryWithoutName(t *testing.T) {
	manifest := "jobs:\n  - fingerprint: abc\n    sha1: sha256:stale\n"
	out, err := patchReleaseManifestChecksums([]byte(manifest), map[string]string{"jobs/": "x"})
	if err != nil {
		t.Fatalf("expected no error for job entry missing 'name': %v", err)
	}
	if !strings.Contains(string(out), "sha256:stale") {
		t.Errorf("expected untouched sha1 to survive when entry has no name field, got: %s", out)
	}
}

func TestPatchReleaseManifestChecksumsEmptyManifest(t *testing.T) {
	out, err := patchReleaseManifestChecksums([]byte(""), map[string]string{"jobs/foo": "x"})
	if err != nil {
		t.Fatalf("expected no error for empty manifest: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty output for empty manifest input, got: %q", out)
	}
}

func TestPatchReleaseManifestChecksumsLicense(t *testing.T) {
	manifest := testManifest + "license:\n  version: license-fp\n  fingerprint: license-fp\n  sha1: sha256:stale-license-sha\n"

	out, err := patchReleaseManifestChecksums([]byte(manifest), map[string]string{"license": "newlicensesha"})
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}

	var doc struct {
		License struct {
			SHA1 string `yaml:"sha1"`
		} `yaml:"license"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.License.SHA1 != "sha256:newlicensesha" {
		t.Errorf("license sha1 = %q, want sha256:newlicensesha", doc.License.SHA1)
	}
}

// --- mappingValue ---

func TestMappingValue(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte("foo: bar\nbaz: 1\n"), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	doc := root.Content[0]

	if v := mappingValue(doc, "foo"); v == nil || v.Value != "bar" {
		t.Errorf("mappingValue(foo) = %+v, want bar", v)
	}
	if v := mappingValue(doc, "missing"); v != nil {
		t.Errorf("mappingValue(missing) = %+v, want nil", v)
	}
	if v := mappingValue(nil, "foo"); v != nil {
		t.Errorf("mappingValue(nil, ...) = %+v, want nil", v)
	}

	var scalarRoot yaml.Node
	if err := yaml.Unmarshal([]byte("just-a-string"), &scalarRoot); err != nil {
		t.Fatalf("unmarshal scalar: %v", err)
	}
	if v := mappingValue(scalarRoot.Content[0], "foo"); v != nil {
		t.Errorf("mappingValue on non-mapping node = %+v, want nil", v)
	}
}

// --- canonicalizeBoshRelease (top-level, file-based) ---

// buildSyntheticBoshRelease writes a minimal but structurally realistic BOSH
// release tarball (release.MF + one job blob + one package blob, both
// deliberately named "registry-data" like the real sample-tile fixture) to
// destPath, with "dirty" timestamps that vary by dirtySeconds so tests can
// simulate two independent `bosh create-release` runs without needing the
// real bosh CLI.
func buildSyntheticBoshRelease(t *testing.T, destPath string, dirtySeconds int) {
	t.Helper()

	base := time.Now()
	dirty := base.Add(time.Duration(dirtySeconds) * time.Second)

	jobBlob := buildTarGz(t, []testTarFile{
		{name: "./", typeflag: tar.TypeDir, mode: 0o755, modTime: dirty},
		{name: "./job.MF", data: "name: registry-data\ntemplates: {}\n", modTime: dirty},
		{name: "./monit", data: "", modTime: dirty},
	}, dirty)

	pkgBlob := buildTarGz(t, []testTarFile{
		{name: "./", typeflag: tar.TypeDir, mode: 0o755, modTime: dirty},
		{name: "./packaging", data: "set -eu\n", modTime: dirty},
	}, dirty)

	manifest := strings.ReplaceAll(strings.ReplaceAll(testManifest,
		"stale-job-sha", sha256Hex(jobBlob)),
		"stale-pkg-sha", sha256Hex(pkgBlob))

	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: manifest, modTime: dirty},
		{name: "jobs/", typeflag: tar.TypeDir, mode: 0o755, modTime: dirty},
		{name: "jobs/registry-data.tgz", data: string(jobBlob), mode: 0o600, modTime: dirty},
		{name: "packages/", typeflag: tar.TypeDir, mode: 0o755, modTime: dirty},
		{name: "packages/registry-data.tgz", data: string(pkgBlob), mode: 0o600, modTime: dirty},
	}, dirty)

	if err := os.WriteFile(destPath, outer, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}
}

func extractEntry(t *testing.T, tarballPath, name string) tarEntry {
	t.Helper()
	data, err := os.ReadFile(tarballPath)
	if err != nil {
		t.Fatalf("read %s: %v", tarballPath, err)
	}
	entries, err := readTarGz(data)
	if err != nil {
		t.Fatalf("readTarGz %s: %v", tarballPath, err)
	}
	e, ok := entries[name]
	if !ok {
		t.Fatalf("entry %s not found in %s", name, tarballPath)
	}
	return e
}

func TestCanonicalizeBoshReleaseDeterministicAcrossDirtyTimestamps(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.tgz")
	pathB := filepath.Join(dir, "b.tgz")

	buildSyntheticBoshRelease(t, pathA, 0)
	buildSyntheticBoshRelease(t, pathB, 5) // simulates a run 5s later

	if err := canonicalizeBoshRelease(pathA); err != nil {
		t.Fatalf("canonicalizeBoshRelease A: %v", err)
	}
	if err := canonicalizeBoshRelease(pathB); err != nil {
		t.Fatalf("canonicalizeBoshRelease B: %v", err)
	}

	dataA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("read A: %v", err)
	}
	dataB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("read B: %v", err)
	}

	if !bytes.Equal(dataA, dataB) {
		t.Error("expected byte-identical canonicalized release tarballs for logically identical inputs with different timestamps")
	}
}

func TestCanonicalizeBoshReleasePatchesChecksumsToMatchRepackedBlobs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	manifestEntryOut := extractEntry(t, path, "release.MF")
	var doc manifestDoc
	if err := yaml.Unmarshal(manifestEntryOut.data, &doc); err != nil {
		t.Fatalf("unmarshal patched manifest: %v", err)
	}

	jobBlob := extractEntry(t, path, "jobs/registry-data.tgz")
	pkgBlob := extractEntry(t, path, "packages/registry-data.tgz")

	wantJobSHA := "sha256:" + sha256Hex(jobBlob.data)
	wantPkgSHA := "sha256:" + sha256Hex(pkgBlob.data)

	if doc.Jobs[0].SHA1 != wantJobSHA {
		t.Errorf("job sha1 = %q, want %q (actual blob checksum)", doc.Jobs[0].SHA1, wantJobSHA)
	}
	if doc.Packages[0].SHA1 != wantPkgSHA {
		t.Errorf("package sha1 = %q, want %q (actual blob checksum)", doc.Packages[0].SHA1, wantPkgSHA)
	}
	// The job and package share a name in this fixture (like the real
	// sample-tile) but have different content, so their patched checksums
	// must not collide.
	if doc.Jobs[0].SHA1 == doc.Packages[0].SHA1 {
		t.Error("job and package checksums collided despite differing content — section/name keying regression")
	}
}

func TestCanonicalizeBoshReleaseCanonicalizesLicenseBlobAndPatchesChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	dirtyA := time.Now()
	licenseBlobA := buildTarGz(t, []testTarFile{{name: "./LICENSE", data: "license text", modTime: dirtyA}}, dirtyA)
	manifestA := testManifest + "license:\n  version: license-fp\n  fingerprint: license-fp\n  sha1: sha256:stale-license-sha\n"
	outerA := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: manifestA, modTime: dirtyA},
		{name: "jobs/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./job.MF", data: "name: registry-data\n", modTime: dirtyA}}, dirtyA)), modTime: dirtyA},
		{name: "packages/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./packaging", data: "set -eu\n", modTime: dirtyA}}, dirtyA)), modTime: dirtyA},
		{name: "license.tgz", data: string(licenseBlobA), mode: 0o600, modTime: dirtyA},
	}, dirtyA)
	if err := os.WriteFile(path, outerA, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	licenseBlobOut := extractEntry(t, path, "license.tgz")
	manifestOut := extractEntry(t, path, "release.MF")

	var doc struct {
		License struct {
			SHA1 string `yaml:"sha1"`
		} `yaml:"license"`
	}
	if err := yaml.Unmarshal(manifestOut.data, &doc); err != nil {
		t.Fatalf("unmarshal patched manifest: %v", err)
	}

	wantSHA := "sha256:" + sha256Hex(licenseBlobOut.data)
	if doc.License.SHA1 != wantSHA {
		t.Errorf("license sha1 = %q, want %q (actual blob checksum)", doc.License.SHA1, wantSHA)
	}
}

func TestCanonicalizeBoshReleaseCanonicalizesUnrecognizedTopLevelBlobs(t *testing.T) {
	// A .tgz blob that isn't under jobs/, packages/, or named license.tgz
	// (e.g. a future bosh-cli addition) must still be byte-canonicalized,
	// even though there's no release.MF field to patch for it.
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.tgz")
	pathB := filepath.Join(dir, "b.tgz")

	build := func(dest string, dirtySeconds int) {
		dirty := time.Now().Add(time.Duration(dirtySeconds) * time.Second)
		mysteryBlob := buildTarGz(t, []testTarFile{{name: "./data", data: "same content", modTime: dirty}}, dirty)
		outer := buildTarGz(t, []testTarFile{
			{name: "release.MF", data: testManifest, modTime: dirty},
			{name: "jobs/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./job.MF", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
			{name: "packages/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./packaging", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
			{name: "extras/mystery.tgz", data: string(mysteryBlob), modTime: dirty},
		}, dirty)
		if err := os.WriteFile(dest, outer, 0o644); err != nil {
			t.Fatalf("write synthetic release: %v", err)
		}
	}

	build(pathA, 0)
	build(pathB, 5)

	if err := canonicalizeBoshRelease(pathA); err != nil {
		t.Fatalf("canonicalizeBoshRelease A: %v", err)
	}
	if err := canonicalizeBoshRelease(pathB); err != nil {
		t.Fatalf("canonicalizeBoshRelease B: %v", err)
	}

	mysteryA := extractEntry(t, pathA, "extras/mystery.tgz")
	mysteryB := extractEntry(t, pathB, "extras/mystery.tgz")
	if !bytes.Equal(mysteryA.data, mysteryB.data) {
		t.Error("expected unrecognized top-level .tgz blob to be canonicalized deterministically too")
	}
}

func TestCanonicalizeBoshReleasePreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("file mode = %o, want %o (original mode should be preserved)", info.Mode().Perm(), 0o640)
	}
}

func TestCanonicalizeBoshReleasePreservesDirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	for _, name := range []string{"jobs/", "packages/"} {
		e := extractEntry(t, path, name)
		if e.header.Typeflag != tar.TypeDir {
			t.Errorf("%s: typeflag = %v, want TypeDir", name, e.header.Typeflag)
		}
		if len(e.data) != 0 {
			t.Errorf("%s: expected no data for directory entry, got %d bytes", name, len(e.data))
		}
	}
}

func TestCanonicalizeBoshReleaseNoLeftoverTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	if _, err := os.Stat(path + ".canonical.tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no leftover temp file, stat returned err=%v", err)
	}
}

func TestCanonicalizeBoshReleaseMissingManifestDoesNotError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	jobBlob := buildTarGz(t, []testTarFile{{name: "./job.MF", data: "name: x\n"}}, time.Now())
	outer := buildTarGz(t, []testTarFile{
		{name: "jobs/x.tgz", data: string(jobBlob)},
	}, time.Now())
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("expected no error when release.MF is absent, got: %v", err)
	}
}

func TestCanonicalizeBoshReleaseErrorsOnUnreadableFile(t *testing.T) {
	dir := t.TempDir() // a directory, not a file — os.ReadFile must fail on it
	if err := canonicalizeBoshRelease(dir); err == nil {
		t.Fatal("expected error when tarball path is a directory, got nil")
	}
}

func TestCanonicalizeBoshReleaseErrorsOnMissingFile(t *testing.T) {
	if err := canonicalizeBoshRelease(filepath.Join(t.TempDir(), "does-not-exist.tgz")); err == nil {
		t.Fatal("expected error for nonexistent tarball path, got nil")
	}
}

func TestCanonicalizeBoshReleaseErrorsOnCorruptNestedBlob(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: testManifest},
		{name: "jobs/registry-data.tgz", data: "not actually a gzip stream"},
	}, time.Now())
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err == nil {
		t.Fatal("expected error when a nested job/package blob is corrupt, got nil")
	}
}
