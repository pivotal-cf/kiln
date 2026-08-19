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

// testTarFile describes one entry for buildTarGz. Per-entry control of
// ModTime/Uid/Gid/order simulates "dirty" bosh-cli output without bosh.
type testTarFile struct {
	name       string
	data       string
	typeflag   byte
	mode       int64
	modTime    time.Time
	uid        int
	gid        int
	uname      string
	linkname   string
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
			Linkname:   f.linkname,
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

// entryBytes reads a tarEntry's spooled data off disk for assertions.
func entryBytes(t *testing.T, e tarEntry) []byte {
	t.Helper()
	if e.path == "" {
		return nil
	}
	data, err := os.ReadFile(e.path)
	if err != nil {
		t.Fatalf("read entry data %s: %v", e.path, err)
	}
	return data
}

// spoolTestEntry writes data to a temp file under dir, for hand-built tarEntry maps.
func spoolTestEntry(t *testing.T, dir, data string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "test-entry-*")
	if err != nil {
		t.Fatalf("create test entry: %v", err)
	}
	if _, err := f.WriteString(data); err != nil {
		t.Fatalf("write test entry: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close test entry: %v", err)
	}
	return f.Name()
}

// mustWriteTarGz runs writeTarGz into an in-memory buffer and returns the bytes.
func mustWriteTarGz(t *testing.T, entries map[string]tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := writeTarGz(entries, &buf); err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}
	return buf.Bytes()
}

// --- readTarGz / writeTarGz ---

func TestReadTarGzRoundTrip(t *testing.T) {
	src := buildTarGz(t, []testTarFile{
		{name: "dir/", typeflag: tar.TypeDir, mode: 0o755, modTime: time.Now(), uid: 501, gid: 20, uname: "someone"},
		{name: "dir/a.txt", data: "hello", modTime: time.Now().Add(-time.Hour), uid: 501, gid: 20, uname: "someone"},
		{name: "b.txt", data: "world"},
	}, time.Now())

	entries, _, err := readTarGz(bytes.NewReader(src), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if got := string(entryBytes(t, entries["dir/a.txt"])); got != "hello" {
		t.Errorf("dir/a.txt content = %q, want %q", got, "hello")
	}
	if got := string(entryBytes(t, entries["b.txt"])); got != "world" {
		t.Errorf("b.txt content = %q, want %q", got, "world")
	}
	if entries["dir/"].header.Typeflag != tar.TypeDir {
		t.Errorf("dir/ typeflag = %v, want TypeDir", entries["dir/"].header.Typeflag)
	}
	if entries["dir/"].path != "" {
		t.Errorf("dir/ path = %q, want empty (directories carry no data)", entries["dir/"].path)
	}
}

func TestReadTarGzInvalidInput(t *testing.T) {
	if _, _, err := readTarGz(strings.NewReader("not a gzip stream"), t.TempDir()); err == nil {
		t.Fatal("expected error for non-gzip input, got nil")
	}
}

func TestReadTarGzErrorsOnDuplicateEntryName(t *testing.T) {
	// entries is keyed by name; a duplicate would otherwise silently
	// overwrite the earlier entry instead of surfacing the anomaly.
	src := buildTarGz(t, []testTarFile{
		{name: "a.txt", data: "first"},
		{name: "a.txt", data: "second"},
	}, time.Now())

	if _, _, err := readTarGz(bytes.NewReader(src), t.TempDir()); err == nil {
		t.Fatal("expected error for duplicate entry name, got nil")
	}
}

func TestReadTarGzCleansUpSpoolFilesOnError(t *testing.T) {
	// The duplicate is caught after the first entry is already spooled;
	// that spool file must still be reported for cleanup.
	src := buildTarGz(t, []testTarFile{
		{name: "a.txt", data: "first"},
		{name: "a.txt", data: "second"},
	}, time.Now())

	dir := t.TempDir()
	_, spooled, err := readTarGz(bytes.NewReader(src), dir)
	if err == nil {
		t.Fatal("expected error for duplicate entry name, got nil")
	}
	if len(spooled) == 0 {
		t.Fatal("expected the first entry's spool file to be reported even though readTarGz failed")
	}
	for _, p := range spooled {
		_ = os.Remove(p)
	}
	leftover, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read spool dir: %v", err)
	}
	if len(leftover) != 0 {
		t.Errorf("expected no leftover spool files after cleanup, got %v", leftover)
	}
}

func TestWriteTarGzCanonicalizesHeaders(t *testing.T) {
	src := buildTarGz(t, []testTarFile{
		{name: "z.txt", data: "z", modTime: time.Now(), uid: 501, gid: 20, uname: "someone"},
		{name: "a.txt", data: "a", modTime: time.Now().Add(-24 * time.Hour), uid: 999, gid: 999, uname: "other"},
	}, time.Now())

	entries, _, err := readTarGz(bytes.NewReader(src), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}

	out := mustWriteTarGz(t, entries)

	gotEntries, _, err := readTarGz(bytes.NewReader(out), t.TempDir())
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
	dir := t.TempDir()
	entries := map[string]tarEntry{
		"z.txt": {header: &tar.Header{Name: "z.txt", Typeflag: tar.TypeReg, Mode: 0o644}, path: spoolTestEntry(t, dir, "z"), size: 1},
		"a.txt": {header: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644}, path: spoolTestEntry(t, dir, "a"), size: 1},
		"m.txt": {header: &tar.Header{Name: "m.txt", Typeflag: tar.TypeReg, Mode: 0o644}, path: spoolTestEntry(t, dir, "m"), size: 1},
	}

	out := mustWriteTarGz(t, entries)

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

	entriesA, _, err := readTarGz(bytes.NewReader(srcA), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz A: %v", err)
	}
	entriesB, _, err := readTarGz(bytes.NewReader(srcB), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz B: %v", err)
	}

	outA := mustWriteTarGz(t, entriesA)
	outB := mustWriteTarGz(t, entriesB)

	if !bytes.Equal(outA, outB) {
		t.Errorf("expected byte-identical canonicalized output for logically identical inputs with different timestamps/order/ownership")
	}
}

func TestWriteTarGzDeterministicRegardlessOfMode(t *testing.T) {
	// Directory entries are created fresh on every build, so their mode
	// tracks the build host's umask. Two umasks, one canonical result.
	filesA := []testTarFile{
		{name: "dir/", typeflag: tar.TypeDir, mode: 0o755},
		{name: "dir/a.txt", data: "content"},
	}
	filesB := []testTarFile{
		{name: "dir/", typeflag: tar.TypeDir, mode: 0o700},
		{name: "dir/a.txt", data: "content"},
	}

	entriesA, _, err := readTarGz(bytes.NewReader(buildTarGz(t, filesA, time.Now())), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz A: %v", err)
	}
	entriesB, _, err := readTarGz(bytes.NewReader(buildTarGz(t, filesB, time.Now())), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz B: %v", err)
	}

	outA := mustWriteTarGz(t, entriesA)
	outB := mustWriteTarGz(t, entriesB)

	if !bytes.Equal(outA, outB) {
		t.Error("expected byte-identical canonicalized output despite differing directory modes (umask)")
	}
}

func TestWriteTarGzPreservesExecutableBitOnRegularFiles(t *testing.T) {
	// Canonicalizing mode must not erase whether a file is executable —
	// e.g. a job's "packaging"/"control" script needs +x to run.
	src := buildTarGz(t, []testTarFile{
		{name: "control", data: "#!/bin/bash\n", mode: 0o755},
		{name: "job.MF", data: "name: x\n", mode: 0o644},
	}, time.Now())

	entries, _, err := readTarGz(bytes.NewReader(src), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}

	out := mustWriteTarGz(t, entries)
	gotEntries, _, err := readTarGz(bytes.NewReader(out), t.TempDir())
	if err != nil {
		t.Fatalf("re-read canonicalized output: %v", err)
	}

	if gotEntries["control"].header.Mode&0o111 == 0 {
		t.Errorf("control mode = %o, want executable bit preserved", gotEntries["control"].header.Mode)
	}
	if gotEntries["job.MF"].header.Mode&0o111 != 0 {
		t.Errorf("job.MF mode = %o, want executable bit not set", gotEntries["job.MF"].header.Mode)
	}
}

func TestWriteTarGzManifestPinIsOrderIndependent(t *testing.T) {
	// The release.MF pin is unconditional, so the emitted order must stay a
	// pure function of the entry set, not of map iteration order.
	dir := t.TempDir()
	build := func() []byte {
		return mustWriteTarGz(t, map[string]tarEntry{
			"b.txt":      {header: &tar.Header{Name: "b.txt", Typeflag: tar.TypeReg, Mode: 0o644}, path: spoolTestEntry(t, dir, "b"), size: 1},
			"release.MF": {header: &tar.Header{Name: "release.MF", Typeflag: tar.TypeReg, Mode: 0o644}, path: spoolTestEntry(t, dir, "m"), size: 1},
			"a.txt":      {header: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644}, path: spoolTestEntry(t, dir, "a"), size: 1},
		})
	}

	first := build()
	for i := 0; i < 5; i++ {
		if !bytes.Equal(first, build()) {
			t.Fatal("writeTarGz output varied across runs over the same entry set")
		}
	}

	gotEntries, _, err := readTarGz(bytes.NewReader(first), t.TempDir())
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if len(gotEntries) != 3 {
		t.Errorf("entry count = %d, want 3", len(gotEntries))
	}
}

func TestWriteTarGzPreservesSetuidSetgidStickyBits(t *testing.T) {
	// setuid/setgid/sticky are real semantic content — no umask can
	// introduce them — so normalizing the permission bits must not drop them.
	src := buildTarGz(t, []testTarFile{
		{name: "suid", data: "x", mode: 0o4755},
		{name: "sgid", data: "x", mode: 0o2644},
		{name: "sticky/", typeflag: tar.TypeDir, mode: 0o1755},
	}, time.Now())

	entries, _, err := readTarGz(bytes.NewReader(src), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}

	gotEntries, _, err := readTarGz(bytes.NewReader(mustWriteTarGz(t, entries)), t.TempDir())
	if err != nil {
		t.Fatalf("re-read canonicalized output: %v", err)
	}

	for name, want := range map[string]int64{"suid": 0o4755, "sgid": 0o2644, "sticky/": 0o1755} {
		if got := gotEntries[name].header.Mode; got != want {
			t.Errorf("%s mode = %o, want %o", name, got, want)
		}
	}
}

func TestWriteTarGzDropsGlobalPAXHeaderEntries(t *testing.T) {
	// tar.Writer rejects a TypeXGlobalHeader with any non-PAX field set, and
	// those records are cleared anyway, so the entry is dropped.
	entries := map[string]tarEntry{
		"pax_global_header": {header: &tar.Header{
			Name:       "pax_global_header",
			Typeflag:   tar.TypeXGlobalHeader,
			PAXRecords: map[string]string{"comment": "from git archive"},
		}},
		"a.txt": {header: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}, path: spoolTestEntry(t, t.TempDir(), "x"), size: 1},
	}

	gotEntries, _, err := readTarGz(bytes.NewReader(mustWriteTarGz(t, entries)), t.TempDir())
	if err != nil {
		t.Fatalf("re-read canonicalized output: %v", err)
	}
	if _, ok := gotEntries["pax_global_header"]; ok {
		t.Error("expected the global PAX header entry to be dropped")
	}
	if _, ok := gotEntries["a.txt"]; !ok {
		t.Error("expected the regular entry to survive")
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

	entries, _, err := readTarGz(bytes.NewReader(src), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz: %v", err)
	}
	// Sanity check the fixture actually carries the attributes we're testing get stripped.
	if len(entries["a.txt"].header.PAXRecords) == 0 {
		t.Fatal("test fixture setup failed: expected PAXRecords to be present before canonicalization")
	}

	out := mustWriteTarGz(t, entries)

	gotEntries, _, err := readTarGz(bytes.NewReader(out), t.TempDir())
	if err != nil {
		t.Fatalf("re-read canonicalized output: %v", err)
	}

	e := gotEntries["a.txt"]
	if len(e.header.PAXRecords) != 0 {
		t.Errorf("expected PAXRecords stripped, got %v", e.header.PAXRecords)
	}
	if len(e.header.Xattrs) != 0 { //nolint:staticcheck // asserting the deprecated field is also cleared
		t.Errorf("expected Xattrs stripped, got %v", e.header.Xattrs) //nolint:staticcheck // deprecated; asserting it is cleared
	}
}

func TestWriteTarGzDeterministicDespiteDifferingPAXRecords(t *testing.T) {
	// Same logical content, different machine-specific xattrs (e.g. SELinux
	// labels), must still canonicalize to identical bytes.
	filesA := []testTarFile{
		{name: "a.txt", data: "content", paxRecords: map[string]string{"SCHILY.xattr.security.selinux": "label-a"}},
	}
	filesB := []testTarFile{
		{name: "a.txt", data: "content", paxRecords: map[string]string{"SCHILY.xattr.security.selinux": "label-b", "extra": "record"}},
	}

	entriesA, _, err := readTarGz(bytes.NewReader(buildTarGz(t, filesA, time.Now())), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz A: %v", err)
	}
	entriesB, _, err := readTarGz(bytes.NewReader(buildTarGz(t, filesB, time.Now())), t.TempDir())
	if err != nil {
		t.Fatalf("readTarGz B: %v", err)
	}

	outA := mustWriteTarGz(t, entriesA)
	outB := mustWriteTarGz(t, entriesB)

	if !bytes.Equal(outA, outB) {
		t.Error("expected byte-identical canonicalized output despite differing PAX records")
	}
}

// --- canonicalizeNestedTarGz ---

func TestCanonicalizeNestedTarGzDetectsContentChange(t *testing.T) {
	dir := t.TempDir()
	blobA := buildTarGz(t, []testTarFile{{name: "job.MF", data: "name: foo\n", modTime: time.Now()}}, time.Now())
	blobB := buildTarGz(t, []testTarFile{{name: "job.MF", data: "name: bar\n", modTime: time.Now()}}, time.Now())

	pathA := filepath.Join(dir, "a.tgz")
	pathB := filepath.Join(dir, "b.tgz")
	if err := os.WriteFile(pathA, blobA, 0o644); err != nil {
		t.Fatalf("write blob A: %v", err)
	}
	if err := os.WriteFile(pathB, blobB, 0o644); err != nil {
		t.Fatalf("write blob B: %v", err)
	}

	dstA, _, _, err := canonicalizeNestedTarGz(pathA, dir)
	if err != nil {
		t.Fatalf("canonicalizeNestedTarGz A: %v", err)
	}
	dstB, _, _, err := canonicalizeNestedTarGz(pathB, dir)
	if err != nil {
		t.Fatalf("canonicalizeNestedTarGz B: %v", err)
	}

	canonA, err := os.ReadFile(dstA)
	if err != nil {
		t.Fatalf("read canonicalized A: %v", err)
	}
	canonB, err := os.ReadFile(dstB)
	if err != nil {
		t.Fatalf("read canonicalized B: %v", err)
	}

	if bytes.Equal(canonA, canonB) {
		t.Error("expected different canonicalized bytes for different content, got identical output")
	}
}

func TestWriteCanonicalBlobRemovesTempFileOnWriteError(t *testing.T) {
	// Regression: dstPath must survive an error return so the deferred
	// cleanup can remove the temp file. `return "", "", err` zeroed it,
	// turning os.Remove into a no-op and leaking a temp file per failure.
	dir := t.TempDir()
	entries := map[string]tarEntry{
		// Typeflag TypeReg with a path to a file that doesn't exist forces
		// writeTarGz's io.Copy step to fail via os.Open.
		"a.txt": {header: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644}, path: filepath.Join(dir, "does-not-exist"), size: 5},
	}

	if _, _, _, err := writeCanonicalBlob(entries, dir); err == nil {
		t.Fatal("expected error when entry's spooled file is missing, got nil")
	}

	leftover, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read spool dir: %v", err)
	}
	if len(leftover) != 0 {
		t.Errorf("expected no leftover canonical-blob temp file after write error, got %v", leftover)
	}
}

func TestCanonicalizeNestedTarGzInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.tgz")
	if err := os.WriteFile(path, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("write garbage blob: %v", err)
	}

	if _, _, _, err := canonicalizeNestedTarGz(path, dir); err == nil {
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

	out, unpatched, err := patchReleaseManifestChecksums([]byte(testManifest), newSHA)
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}
	if len(unpatched) != 0 {
		t.Errorf("unpatched = %v, want none", unpatched)
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
	// Regression: a job and a package sharing a name (as in sample-tile) must
	// be keyed by section, not name alone.
	newSHA := map[string]string{
		"jobs/registry-data":     "jobhash",
		"packages/registry-data": "pkghash",
	}

	out, unpatched, err := patchReleaseManifestChecksums([]byte(testManifest), newSHA)
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}
	if len(unpatched) != 0 {
		t.Errorf("unpatched = %v, want none", unpatched)
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
	out, unpatched, err := patchReleaseManifestChecksums([]byte(testManifest), map[string]string{})
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}
	if len(unpatched) != 0 {
		t.Errorf("unpatched = %v, want none for an empty newSHA", unpatched)
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
	out, unpatched, err := patchReleaseManifestChecksums([]byte(minimal), map[string]string{"jobs/foo": "x"})
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums should not error when jobs/packages keys are absent: %v", err)
	}
	if !strings.Contains(string(out), "empty-release") {
		t.Errorf("expected manifest content preserved, got: %s", out)
	}
	// The blob was repacked but nothing recorded its checksum: the caller
	// must hear about it rather than shipping a mismatched release.
	if len(unpatched) != 1 || unpatched[0] != "jobs/foo" {
		t.Errorf("unpatched = %v, want [jobs/foo]", unpatched)
	}
}

func TestPatchReleaseManifestChecksumsEntryWithoutName(t *testing.T) {
	manifest := "jobs:\n  - fingerprint: abc\n    sha1: sha256:stale\n"
	out, unpatched, err := patchReleaseManifestChecksums([]byte(manifest), map[string]string{"jobs/": "x"})
	if err != nil {
		t.Fatalf("expected no error for job entry missing 'name': %v", err)
	}
	if !strings.Contains(string(out), "sha256:stale") {
		t.Errorf("expected untouched sha1 to survive when entry has no name field, got: %s", out)
	}
	if len(unpatched) != 1 || unpatched[0] != "jobs/" {
		t.Errorf("unpatched = %v, want [jobs/]", unpatched)
	}
}

func TestPatchReleaseManifestChecksumsEmptyManifest(t *testing.T) {
	out, unpatched, err := patchReleaseManifestChecksums([]byte(""), map[string]string{"jobs/foo": "x"})
	if err != nil {
		t.Fatalf("expected no error for empty manifest: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty output for empty manifest input, got: %q", out)
	}
	if len(unpatched) != 1 || unpatched[0] != "jobs/foo" {
		t.Errorf("unpatched = %v, want [jobs/foo]", unpatched)
	}
}

func TestPatchReleaseManifestChecksumsLicense(t *testing.T) {
	manifest := testManifest + "license:\n  version: license-fp\n  fingerprint: license-fp\n  sha1: sha256:stale-license-sha\n"

	out, unpatched, err := patchReleaseManifestChecksums([]byte(manifest), map[string]string{"license": "newlicensesha"})
	if err != nil {
		t.Fatalf("patchReleaseManifestChecksums: %v", err)
	}
	if len(unpatched) != 0 {
		t.Errorf("unpatched = %v, want none", unpatched)
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

// buildSyntheticBoshRelease writes a minimal release tarball (release.MF +
// one job + one package blob), with timestamps offset by dirtySeconds to
// simulate two independent `bosh create-release` runs without bosh.
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
	f, err := os.Open(tarballPath)
	if err != nil {
		t.Fatalf("open %s: %v", tarballPath, err)
	}
	defer func() { _ = f.Close() }()
	entries, _, err := readTarGz(f, t.TempDir())
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
	if err := yaml.Unmarshal(entryBytes(t, manifestEntryOut), &doc); err != nil {
		t.Fatalf("unmarshal patched manifest: %v", err)
	}

	jobBlob := extractEntry(t, path, "jobs/registry-data.tgz")
	pkgBlob := extractEntry(t, path, "packages/registry-data.tgz")

	wantJobSHA := "sha256:" + sha256Hex(entryBytes(t, jobBlob))
	wantPkgSHA := "sha256:" + sha256Hex(entryBytes(t, pkgBlob))

	if doc.Jobs[0].SHA1 != wantJobSHA {
		t.Errorf("job sha1 = %q, want %q (actual blob checksum)", doc.Jobs[0].SHA1, wantJobSHA)
	}
	if doc.Packages[0].SHA1 != wantPkgSHA {
		t.Errorf("package sha1 = %q, want %q (actual blob checksum)", doc.Packages[0].SHA1, wantPkgSHA)
	}
	// Job and package share a name here (as in sample-tile) but differ in
	// content, so their patched checksums must not collide.
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
	if err := yaml.Unmarshal(entryBytes(t, manifestOut), &doc); err != nil {
		t.Fatalf("unmarshal patched manifest: %v", err)
	}

	wantSHA := "sha256:" + sha256Hex(entryBytes(t, licenseBlobOut))
	if doc.License.SHA1 != wantSHA {
		t.Errorf("license sha1 = %q, want %q (actual blob checksum)", doc.License.SHA1, wantSHA)
	}
}

func TestCanonicalizeBoshReleaseLeavesUnrecognizedBlobsByteIdentical(t *testing.T) {
	// A .tgz release.MF doesn't describe has no sha1 we can patch, so
	// repacking it would silently invalidate any recorded checksum. Its bytes
	// must survive untouched; its outer tar header is still normalized.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	dirty := time.Now()
	mysteryBlob := buildTarGz(t, []testTarFile{{name: "./data", data: "same content", modTime: dirty}}, dirty)
	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: testManifest, modTime: dirty},
		{name: "jobs/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./job.MF", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
		{name: "packages/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./packaging", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
		{name: "extras/mystery.tgz", data: string(mysteryBlob), modTime: dirty},
	}, dirty)
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	got := entryBytes(t, extractEntry(t, path, "extras/mystery.tgz"))
	if !bytes.Equal(got, mysteryBlob) {
		t.Error("expected an unrecognized nested .tgz to be passed through byte-for-byte")
	}
}

func TestCanonicalizeBoshReleaseIgnoresAppleDoubleSidecars(t *testing.T) {
	// bosh <= 7.0.1 emits AppleDouble sidecars on macOS: jobs/._x.tgz sits
	// next to jobs/x.tgz and is not a gzip stream. release.MF never mentions
	// it, so it must pass through rather than be fed to gzip.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	dirty := time.Now()
	const sidecarData = "\x00\x05\x16\x07not-a-gzip-AppleDouble-blob"
	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: testManifest, modTime: dirty},
		{name: "._release.MF", data: sidecarData, modTime: dirty},
		{name: "jobs/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./job.MF", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
		{name: "jobs/._registry-data.tgz", data: sidecarData, modTime: dirty},
		{name: "packages/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./packaging", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
		{name: "packages/._registry-data.tgz", data: sidecarData, modTime: dirty},
	}, dirty)
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("expected AppleDouble sidecars to be ignored, got: %v", err)
	}

	for _, name := range []string{"._release.MF", "jobs/._registry-data.tgz", "packages/._registry-data.tgz"} {
		if got := string(entryBytes(t, extractEntry(t, path, name))); got != sidecarData {
			t.Errorf("%s: sidecar data = %q, want it passed through unchanged", name, got)
		}
	}
}

func TestCanonicalizeBoshReleaseIsIdempotent(t *testing.T) {
	// A second pass must be a no-op — the cheapest guard over the whole
	// header-normalization surface.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first pass: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second pass: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("expected canonicalizeBoshRelease to be idempotent, second pass changed the bytes")
	}
}

func TestCanonicalizeBoshReleasePatchesCompiledPackages(t *testing.T) {
	// `bosh export-release` output carries compiled_packages/<name>.tgz with
	// their own sha1; repacking without patching yields a rejected release.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	dirty := time.Now()
	compiledBlob := buildTarGz(t, []testTarFile{{name: "./compiled", data: "binary", modTime: dirty}}, dirty)
	manifest := "name: compiled-release\nversion: 1.0.0\ncompiled_packages:\n  - name: registry-data\n    fingerprint: fp\n    sha1: sha256:stale-compiled-sha\n"
	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: manifest, modTime: dirty},
		{name: "compiled_packages/registry-data.tgz", data: string(compiledBlob), modTime: dirty},
	}, dirty)
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	var doc struct {
		CompiledPackages []struct {
			SHA1 string `yaml:"sha1"`
		} `yaml:"compiled_packages"`
	}
	if err := yaml.Unmarshal(entryBytes(t, extractEntry(t, path, "release.MF")), &doc); err != nil {
		t.Fatalf("unmarshal patched manifest: %v", err)
	}

	want := "sha256:" + sha256Hex(entryBytes(t, extractEntry(t, path, "compiled_packages/registry-data.tgz")))
	if len(doc.CompiledPackages) != 1 || doc.CompiledPackages[0].SHA1 != want {
		t.Errorf("compiled_packages sha1 = %+v, want %q", doc.CompiledPackages, want)
	}
}

func TestCanonicalizeBoshReleaseErrorsWhenRepackedBlobHasNoManifestChecksum(t *testing.T) {
	// A jobs/ entry with no sha1 field: the blob is repacked, so shipping it
	// unpatched leaves nothing verifiable. Fail here, not at upload time.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	dirty := time.Now()
	manifest := "name: r\nversion: 1.0.0\njobs:\n  - name: registry-data\n    fingerprint: fp\n"
	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: manifest, modTime: dirty},
		{name: "jobs/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./job.MF", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
	}, dirty)
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}

	err := canonicalizeBoshRelease(path)
	if err == nil {
		t.Fatal("expected an error when a repacked blob has no sha1 to patch, got nil")
	}
	if !strings.Contains(err.Error(), "jobs/registry-data") {
		t.Errorf("error should name the unpatched blob, got: %v", err)
	}
}

func TestCanonicalizeBoshReleaseEmitsManifestFirst(t *testing.T) {
	// kiln's readers scan for release.MF and stop at the first hit. Sorted
	// alphabetically it lands last, forcing a full multi-GB decompress.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer func() { _ = gz.Close() }()
	hdr, err := tar.NewReader(gz).Next()
	if err != nil {
		t.Fatalf("read first entry: %v", err)
	}
	if hdr.Name != "release.MF" {
		t.Errorf("first entry = %q, want release.MF", hdr.Name)
	}
}

func TestCanonicalizeBoshReleaseSkipsNonRegularTgzEntries(t *testing.T) {
	// A symlink whose name ends in .tgz has no spooled data to open; it must
	// pass through as a symlink rather than crashing the repack.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	dirty := time.Now()
	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", data: testManifest, modTime: dirty},
		{name: "jobs/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./job.MF", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
		{name: "packages/registry-data.tgz", data: string(buildTarGz(t, []testTarFile{{name: "./packaging", data: "x", modTime: dirty}}, dirty)), modTime: dirty},
		{name: "packages/alias.tgz", typeflag: tar.TypeSymlink, linkname: "registry-data.tgz", modTime: dirty},
	}, dirty)
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write synthetic release: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	e := extractEntry(t, path, "packages/alias.tgz")
	if e.header.Typeflag != tar.TypeSymlink || e.header.Linkname != "registry-data.tgz" {
		t.Errorf("symlink entry mangled: typeflag=%v linkname=%q", e.header.Typeflag, e.header.Linkname)
	}
}

func TestManifestBlobs(t *testing.T) {
	manifest := testManifest +
		"compiled_packages:\n  - name: compiled-thing\n    sha1: sha256:x\n" +
		"license:\n  fingerprint: lf\n  sha1: sha256:y\n"

	got, err := manifestBlobs([]byte(manifest))
	if err != nil {
		t.Fatalf("manifestBlobs: %v", err)
	}
	want := map[string]string{
		"jobs/registry-data.tgz":               "jobs/registry-data",
		"packages/registry-data.tgz":           "packages/registry-data",
		"compiled_packages/compiled-thing.tgz": "compiled_packages/compiled-thing",
		"license.tgz":                          "license",
	}
	if len(got) != len(want) {
		t.Fatalf("manifestBlobs = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("manifestBlobs[%q] = %q, want %q", k, got[k], v)
		}
	}

	// No license key means no license blob to look for.
	noLicense, err := manifestBlobs([]byte(testManifest))
	if err != nil {
		t.Fatalf("manifestBlobs: %v", err)
	}
	if _, ok := noLicense["license.tgz"]; ok {
		t.Error("expected no license.tgz blob when release.MF has no license key")
	}

	// An empty document has nothing to index, and must not error.
	empty, err := manifestBlobs(nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("manifestBlobs(nil) = (%v, %v), want (empty, nil)", empty, err)
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
		if len(entryBytes(t, e)) != 0 {
			t.Errorf("%s: expected no data for directory entry, got %d bytes", name, len(entryBytes(t, e)))
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

	// The output temp name is randomized (os.CreateTemp), so glob on
	// "release.tgz.*" rather than one exact name.
	matches, err := filepath.Glob(filepath.Join(dir, "release.tgz.*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no leftover temp file, found %v", matches)
	}
}

func TestCanonicalizeBoshReleaseNoLeftoverSpoolFiles(t *testing.T) {
	// Only the final canonicalized release should remain in dir.
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")
	buildSyntheticBoshRelease(t, path, 0)

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("canonicalizeBoshRelease: %v", err)
	}

	leftover, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(leftover) != 1 || leftover[0].Name() != "release.tgz" {
		names := make([]string, len(leftover))
		for i, e := range leftover {
			names[i] = e.Name()
		}
		t.Errorf("expected only release.tgz in %s after canonicalization, got %v", dir, names)
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

	// With no manifest there is no checksum to patch and none to
	// invalidate, so blobs are passed through rather than repacked.
	if got := entryBytes(t, extractEntry(t, path, "jobs/x.tgz")); !bytes.Equal(got, jobBlob) {
		t.Error("expected the blob to be passed through byte-for-byte when release.MF is absent")
	}
}

func TestCanonicalizeBoshReleaseNonRegularManifestDoesNotError(t *testing.T) {
	// A non-regular release.MF (path == "") has nothing to read or patch, so
	// it passes through rather than erroring on os.ReadFile("").
	dir := t.TempDir()
	path := filepath.Join(dir, "release.tgz")

	jobBlob := buildTarGz(t, []testTarFile{{name: "./job.MF", data: "name: x\n"}}, time.Now())
	outer := buildTarGz(t, []testTarFile{
		{name: "release.MF", typeflag: tar.TypeSymlink},
		{name: "jobs/x.tgz", data: string(jobBlob)},
	}, time.Now())
	if err := os.WriteFile(path, outer, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := canonicalizeBoshRelease(path); err != nil {
		t.Fatalf("expected no error when release.MF is not a regular file, got: %v", err)
	}

	e := extractEntry(t, path, "release.MF")
	if e.header.Typeflag != tar.TypeSymlink {
		t.Errorf("release.MF typeflag = %v, want unchanged TypeSymlink", e.header.Typeflag)
	}
}

func TestCanonicalizeBoshReleaseErrorsOnUnreadableFile(t *testing.T) {
	dir := t.TempDir() // a directory, not a file — os.Open must fail on it
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
