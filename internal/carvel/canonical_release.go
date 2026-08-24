package carvel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Repacks a bosh-cli release tarball and its nested job/package blobs with
// fixed timestamps/ownership, then patches release.MF checksums to match.

var canonicalEpoch = time.Unix(0, 0).UTC()
var manifestSections = []string{"jobs", "packages", "compiled_packages"}

const canonicalGzipLevel = gzip.DefaultCompression
const manifestFileName = "release.MF"

type tarEntry struct {
	header *tar.Header
	path   string
	size   int64
}

func canonicalizeBoshRelease(tarballPath, productVersion string) (string, error) {
	info, err := os.Stat(tarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat bosh release tarball: %w", err)
	}
	origMode := info.Mode()

	spoolDir := filepath.Dir(tarballPath)
	spoolFiles := map[string]struct{}{}
	track := func(paths ...string) {
		for _, p := range paths {
			spoolFiles[p] = struct{}{}
		}
	}
	untrack := func(path string) {
		delete(spoolFiles, path)
	}
	defer func() {
		for p := range spoolFiles {
			_ = os.Remove(p)
		}
	}()

	f, err := os.Open(tarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to read bosh release tarball: %w", err)
	}
	entries, spooled, err := readTarGz(f, spoolDir)
	track(spooled...)
	_ = f.Close()
	if err != nil {
		return "", fmt.Errorf("failed to parse bosh release tarball: %w", err)
	}

	manifestEntry, hasManifest := entries[manifestFileName]
	hasManifest = hasManifest && manifestEntry.header.Typeflag == tar.TypeReg
	if !hasManifest {
		return "", fmt.Errorf("release.MF not found or not a regular file; cannot derive a content-based release version")
	}

	manifestData, err := os.ReadFile(manifestEntry.path)
	if err != nil {
		return "", fmt.Errorf("failed to read release.MF: %w", err)
	}
	blobs, err := manifestBlobs(manifestData)
	if err != nil {
		return "", fmt.Errorf("failed to parse release.MF: %w", err)
	}

	newSHA := map[string]string{} // release.MF checksum key -> new sha256 hex

	blobNames := make([]string, 0, len(blobs))
	for name := range blobs {
		blobNames = append(blobNames, name)
	}
	sort.Strings(blobNames)

	for _, name := range blobNames {
		entry, ok := entries[name]
		if !ok || entry.header.Typeflag != tar.TypeReg {
			continue
		}

		dstPath, size, hexSum, err := canonicalizeNestedTarGz(entry.path, spoolDir)
		if err != nil {
			return "", fmt.Errorf("failed to canonicalize %s: %w", name, err)
		}
		track(dstPath)
		// Remove now, not at the final defer, so disk stays bounded to blobs in flight.
		_ = os.Remove(entry.path)
		untrack(entry.path)

		entries[name] = tarEntry{header: entry.header, path: dstPath, size: size}
		newSHA[blobs[name]] = hexSum
	}

	patched, unpatched, err := patchReleaseManifestChecksums(manifestData, newSHA)
	if err != nil {
		return "", fmt.Errorf("failed to patch release.MF: %w", err)
	}

	if len(unpatched) > 0 {
		return "", fmt.Errorf("repacked blob(s) %s have no sha1 field in release.MF to patch; "+
			"canonicalizing them would leave the release checksums inconsistent",
			strings.Join(unpatched, ", "))
	}

	finalManifest, releaseVersion, err := finalizeReleaseVersion(patched, productVersion)
	if err != nil {
		return "", fmt.Errorf("failed to derive content-based release version: %w", err)
	}

	mf, err := os.CreateTemp(spoolDir, "canonical-manifest-*")
	if err != nil {
		return "", fmt.Errorf("failed to write release.MF: %w", err)
	}
	track(mf.Name())
	if _, err := mf.Write(finalManifest); err != nil {
		_ = mf.Close()
		return "", fmt.Errorf("failed to write release.MF: %w", err)
	}
	if err := mf.Close(); err != nil {
		return "", fmt.Errorf("failed to write release.MF: %w", err)
	}
	_ = os.Remove(manifestEntry.path)
	untrack(manifestEntry.path)
	entries[manifestFileName] = tarEntry{header: manifestEntry.header, path: mf.Name(), size: int64(len(finalManifest))}

	out, err := os.CreateTemp(spoolDir, filepath.Base(tarballPath)+".canonical-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to write canonical bosh release tarball: %w", err)
	}
	tmp := out.Name()
	if err := writeTarGz(entries, out); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to write canonical bosh release tarball: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to write canonical bosh release tarball: %w", err)
	}
	if err := os.Chmod(tmp, origMode); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to set canonical bosh release tarball permissions: %w", err)
	}
	if err := os.Rename(tmp, tarballPath); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to replace bosh release tarball with canonical version: %w", err)
	}
	return releaseVersion, nil
}

// finalizeReleaseVersion derives a content-addressed release version and
// writes it into the manifest's version field. commit_hash and
// uncommitted_changes vary with git state even when release content is
// identical, so they are excluded from the fingerprint (but left in the
// output manifest for provenance) -- otherwise two byte-identical releases
// built from different commits would get different versions, and two
// releases with different bytes could end up sharing a version.
func finalizeReleaseVersion(manifest []byte, productVersion string) ([]byte, string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(manifest, &root); err != nil {
		return nil, "", err
	}
	if len(root.Content) == 0 {
		return nil, "", fmt.Errorf("release.MF is empty; cannot derive a content-based version")
	}
	doc := root.Content[0]

	versionNode := mappingValue(doc, "version")
	commitHashNode := mappingValue(doc, "commit_hash")
	uncommittedNode := mappingValue(doc, "uncommitted_changes")

	// Mask to a fixed scalar representation (tag+style+value), not just
	// value, so e.g. a `null` vs an empty string for the same field can't
	// produce different masked bytes and thus different fingerprints.
	mask := func(n *yaml.Node) (restore func()) {
		if n == nil {
			return func() {}
		}
		origTag, origStyle, origValue := n.Tag, n.Style, n.Value
		n.Tag, n.Style, n.Value = "!!str", 0, ""
		return func() { n.Tag, n.Style, n.Value = origTag, origStyle, origValue }
	}
	restoreCommitHash := mask(commitHashNode)
	restoreUncommitted := mask(uncommittedNode)
	mask(versionNode)

	masked, err := encodeYAML(&root)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render manifest for fingerprinting: %w", err)
	}
	sum := sha256.Sum256(masked)
	fingerprint := hex.EncodeToString(sum[:])[:12]
	version := buildReleaseVersion(productVersion, fingerprint)

	restoreCommitHash()
	restoreUncommitted()
	if versionNode != nil {
		versionNode.Tag, versionNode.Style, versionNode.Value = "!!str", 0, version
	}

	final, err := encodeYAML(&root)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render finalized manifest: %w", err)
	}
	return final, version, nil
}

func encodeYAML(root *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func manifestBlobs(manifest []byte) (map[string]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(manifest, &root); err != nil {
		return nil, err
	}
	blobs := map[string]string{}
	if len(root.Content) == 0 {
		return blobs, nil
	}
	doc := root.Content[0]

	for _, section := range manifestSections {
		seq := mappingValue(doc, section)
		if seq == nil {
			continue
		}
		for _, item := range seq.Content {
			nameNode := mappingValue(item, "name")
			if nameNode == nil || nameNode.Value == "" {
				continue
			}
			blobs[section+"/"+nameNode.Value+".tgz"] = section + "/" + nameNode.Value
		}
	}
	if mappingValue(doc, "license") != nil {
		blobs["license.tgz"] = "license"
	}
	return blobs, nil
}

func canonicalizeNestedTarGz(srcPath, spoolDir string) (dstPath string, size int64, sha256hex string, err error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return "", 0, "", err
	}
	defer func() { _ = f.Close() }()

	entries, spooled, err := readTarGz(f, spoolDir)
	defer func() {
		for _, p := range spooled {
			_ = os.Remove(p)
		}
	}()
	if err != nil {
		return "", 0, "", err
	}

	return writeCanonicalBlob(entries, spoolDir)
}

type countingWriter struct{ n int64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

func writeCanonicalBlob(entries map[string]tarEntry, spoolDir string) (dstPath string, size int64, sha256hex string, err error) {
	dst, err := os.CreateTemp(spoolDir, "canonical-blob-*.tmp")
	if err != nil {
		return "", 0, "", err
	}
	dstPath = dst.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(dstPath)
		}
	}()

	h := sha256.New()
	var cw countingWriter

	if err = writeTarGz(entries, io.MultiWriter(dst, h, &cw)); err != nil {
		_ = dst.Close()
		return
	}
	if err = dst.Close(); err != nil {
		return
	}

	return dstPath, cw.n, hex.EncodeToString(h.Sum(nil)), nil
}

func readTarGz(r io.Reader, spoolDir string) (map[string]tarEntry, []string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	entries := map[string]tarEntry{}
	var spoolPaths []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, spoolPaths, err
		}

		if _, exists := entries[hdr.Name]; exists {
			return nil, spoolPaths, fmt.Errorf("duplicate entry %q in tar archive", hdr.Name)
		}

		var path string
		var size int64
		if hdr.Typeflag == tar.TypeReg {
			tmp, err := os.CreateTemp(spoolDir, "spool-*")
			if err != nil {
				return nil, spoolPaths, err
			}
			path = tmp.Name()
			spoolPaths = append(spoolPaths, path)

			n, copyErr := io.Copy(tmp, tr)
			closeErr := tmp.Close()
			if copyErr != nil {
				return nil, spoolPaths, copyErr
			}
			if closeErr != nil {
				return nil, spoolPaths, closeErr
			}
			size = n
		}

		h := *hdr
		entries[hdr.Name] = tarEntry{header: &h, path: path, size: size}
	}
	return entries, spoolPaths, nil
}

func writeTarGz(entries map[string]tarEntry, w io.Writer) error {
	names := make([]string, 0, len(entries))
	for name := range entries {
		if name == manifestFileName {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if _, ok := entries[manifestFileName]; ok {
		names = append([]string{manifestFileName}, names...)
	}

	gw, err := gzip.NewWriterLevel(w, canonicalGzipLevel)
	if err != nil {
		return err
	}
	gw.ModTime = canonicalEpoch

	tw := tar.NewWriter(gw)
	for _, name := range names {
		e := entries[name]

		if e.header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}

		hdr := *e.header
		hdr.Name = name
		hdr.ModTime = canonicalEpoch
		hdr.AccessTime = time.Time{}
		hdr.ChangeTime = time.Time{}
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Uname = ""
		hdr.Gname = ""
		hdr.Size = e.size

		switch hdr.Typeflag {
		case tar.TypeDir:
			hdr.Mode = hdr.Mode&0o7000 | 0o755
		case tar.TypeReg:
			if hdr.Mode&0o111 != 0 {
				hdr.Mode = hdr.Mode&0o7000 | 0o755
			} else {
				hdr.Mode = hdr.Mode&0o7000 | 0o644
			}
		}

		hdr.PAXRecords = nil
		hdr.Xattrs = nil //nolint:staticcheck // deprecated; cleared alongside PAXRecords
		hdr.Format = tar.FormatUnknown

		if err := tw.WriteHeader(&hdr); err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg {
			data, err := os.Open(e.path)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, data)
			_ = data.Close()
			if err != nil {
				return err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

func patchReleaseManifestChecksums(manifest []byte, newSHA map[string]string) (patched []byte, unpatched []string, err error) {
	remaining := make(map[string]struct{}, len(newSHA))
	for key := range newSHA {
		remaining[key] = struct{}{}
	}
	missing := func() []string {
		keys := make([]string, 0, len(remaining))
		for key := range remaining {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys
	}

	var root yaml.Node
	if err := yaml.Unmarshal(manifest, &root); err != nil {
		return nil, nil, err
	}
	if len(root.Content) == 0 {
		return manifest, missing(), nil
	}
	doc := root.Content[0]

	for _, section := range manifestSections {
		seq := mappingValue(doc, section)
		if seq == nil {
			continue
		}
		for _, item := range seq.Content {
			nameNode := mappingValue(item, "name")
			if nameNode == nil {
				continue
			}
			key := section + "/" + nameNode.Value
			newHash, ok := newSHA[key]
			if !ok {
				continue
			}
			if shaNode := mappingValue(item, "sha1"); shaNode != nil {
				shaNode.Value = "sha256:" + newHash
				delete(remaining, key)
			}
		}
	}

	if newHash, ok := newSHA["license"]; ok {
		if license := mappingValue(doc, "license"); license != nil {
			if shaNode := mappingValue(license, "sha1"); shaNode != nil {
				shaNode.Value = "sha256:" + newHash
				delete(remaining, "license")
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), missing(), nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
